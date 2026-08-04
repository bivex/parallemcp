package tools

import (
	"context"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

// execFakeRunner is a fake parallels.Runner. It answers `prlctl list -i` with a
// minimal VMInfo carrying infoOS (so the handler's OS detection works) and routes
// everything else to the canned result/error, capturing the full argument vector.
type execFakeRunner struct {
	res    *parallels.CmdResult
	err    error
	infoOS string // OS reported by `prlctl list -i`
	args   []string
	vm     string
}

func (f *execFakeRunner) Run(_ context.Context, _ string, a ...string) (*parallels.CmdResult, error) {
	if len(a) > 0 && a[0] == "list" {
		js := fmt.Sprintf(`[{"ID":"x","OS":%q,"State":"running"}]`, f.infoOS)
		return &parallels.CmdResult{Stdout: js}, nil
	}
	f.args = append([]string(nil), a...)
	if len(a) >= 2 {
		f.vm = a[1]
	}
	return f.res, f.err
}

func newExecTools(res *parallels.CmdResult, err error) (*Tools, *execFakeRunner) {
	return newExecToolsOS(res, err, "")
}

func newExecToolsOS(res *parallels.CmdResult, err error, infoOS string) (*Tools, *execFakeRunner) {
	r := &execFakeRunner{res: res, err: err, infoOS: infoOS}
	return &Tools{cli: &parallels.Client{Run: r}}, r
}

// decodeUTF16LE reverses the PowerShell -EncodedCommand encoding for assertions.
func decodeUTF16LE(b64 string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		return "", err
	}
	runes := make([]rune, len(raw)/2)
	for i := 0; i*2+1 < len(raw); i++ {
		runes[i] = rune(raw[2*i]) | rune(raw[2*i+1])<<8
	}
	return string(runes), nil
}

func textOf(t *testing.T, res *mcp.CallToolResult) string {
	t.Helper()
	if res == nil || len(res.Content) == 0 {
		t.Fatal("empty tool result")
	}
	tc, ok := res.Content[0].(*mcp.TextContent)
	if !ok {
		t.Fatalf("content[0] is %T, want *mcp.TextContent", res.Content[0])
	}
	return tc.Text
}

func TestVMExecRequiresVMAndCommand(t *testing.T) {
	tools, _ := newExecTools(&parallels.CmdResult{}, nil)

	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{})
	if !res.IsError {
		t.Fatal("expected IsError for empty input")
	}
	if !strings.Contains(textOf(t, res), "required") {
		t.Errorf("missing 'required' message: %s", textOf(t, res))
	}

	res, _, _ = tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{VM: "vm"})
	if !res.IsError || !strings.Contains(textOf(t, res), "required") {
		t.Errorf("expected required error for missing command: %s", textOf(t, res))
	}
}

func TestVMExecSuccessRenderer(t *testing.T) {
	tools, run := newExecTools(&parallels.CmdResult{
		Stdout:   "hello\n",
		Stderr:   "",
		ExitCode: 0,
	}, nil)

	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "Ubuntu", Command: "echo hello",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	md := textOf(t, res)
	for _, want := range []string{"echo hello", "Ubuntu", "exit 0", "hello"} {
		if !strings.Contains(md, want) {
			t.Errorf("rendered output missing %q:\n%s", want, md)
		}
	}
	if run.vm != "Ubuntu" || run.args[len(run.args)-1] != "echo hello" {
		t.Errorf("command not forwarded correctly: vm=%q args=%v", run.vm, run.args)
	}
}

func TestVMExecRendersStderrAndNonZeroExit(t *testing.T) {
	tools, _ := newExecTools(&parallels.CmdResult{
		Stdout:   "",
		Stderr:   "command not found: frobnicate",
		ExitCode: 127,
	}, nil)

	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "vm", Command: "frobnicate",
	})
	if res.IsError {
		t.Fatalf("guest non-zero exit should not be a tool error: %s", textOf(t, res))
	}
	md := textOf(t, res)
	if !strings.Contains(md, "exit 127") {
		t.Errorf("missing exit 127: %s", md)
	}
	if !strings.Contains(md, "command not found") {
		t.Errorf("missing stderr: %s", md)
	}
}

func TestVMExecNoOutput(t *testing.T) {
	tools, _ := newExecTools(&parallels.CmdResult{ExitCode: 0}, nil)
	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "vm", Command: "true",
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	if !strings.Contains(textOf(t, res), "(no output)") {
		t.Errorf("expected '(no output)' marker: %s", textOf(t, res))
	}
}

func TestVMExecPropagatesExecError(t *testing.T) {
	err := &parallels.ExitError{
		Bin:    "prlctl",
		Args:   []string{"exec", "vm", "/bin/sh", "-lc", "x"},
		Code:   1,
		Stderr: "Parallels Tools are not installed",
		Err:    context.DeadlineExceeded,
	}
	tools, _ := newExecTools(&parallels.CmdResult{ExitCode: 1, Stderr: err.Stderr}, err)

	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "vm", Command: "x",
	})
	if !res.IsError {
		t.Fatal("expected IsError for exec failure")
	}
	md := textOf(t, res)
	if !strings.Contains(md, "exec command") || !strings.Contains(md, "Parallels Tools") {
		t.Errorf("error not surfaced: %s", md)
	}
}

// TestVMExecDetectsWindows verifies the handler detects a Windows guest and
// routes the command through cmd.exe /c.
func TestVMExecDetectsWindows(t *testing.T) {
	script := "dir"
	tools, run := newExecToolsOS(&parallels.CmdResult{ExitCode: 0, Stdout: "Dir\n"}, nil, "win-11")

	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "Windows 11", Command: script,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	a := run.args
	if len(a) != 5 {
		t.Fatalf("arg count = %d, want 5: %v", len(a), a)
	}
	if a[0] != "exec" || a[1] != "Windows 11" || a[2] != "cmd.exe" || a[3] != "/c" || a[4] != "dir" {
		t.Errorf("unexpected args: %v", a)
	}
	if !strings.Contains(textOf(t, res), "Dir") {
		t.Errorf("stdout not rendered: %s", textOf(t, res))
	}
}

// TestVMExecWindowsCommandForwardedVerbatim checks the client-side guarantee for
// the legacy Windows string form (no OS detected): the command reaches the guest
// shell verbatim, never mangled by the wrapper.
func TestVMExecWindowsCommandForwardedVerbatim(t *testing.T) {
	cmd := `cmd /c "echo %USERNAME% & dir C:\Users\Admin"`
	tools, run := newExecTools(&parallels.CmdResult{ExitCode: 0}, nil)
	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "Windows 11", Command: cmd,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	if run.args[len(run.args)-1] != cmd {
		t.Errorf("Windows command mangled:\n got  %q\n want %q", run.args[len(run.args)-1], cmd)
	}
}
