package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

// execFakeRunner is a fake parallels.Runner that ignores the command line and
// returns a canned result/error, so the vm_exec handler can be tested without a
// Parallels install.
type execFakeRunner struct {
	res *parallels.CmdResult
	err error
	got string // command actually passed to Exec (last arg)
	vm  string
}

func (f *execFakeRunner) Run(_ context.Context, _ string, args ...string) (*parallels.CmdResult, error) {
	if len(args) >= 2 {
		f.vm = args[1]
	}
	if len(args) >= 5 {
		f.got = args[4]
	}
	return f.res, f.err
}

func newExecTools(res *parallels.CmdResult, err error) (*Tools, *execFakeRunner) {
	r := &execFakeRunner{res: res, err: err}
	return &Tools{cli: &parallels.Client{Run: r}}, r
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
	if run.vm != "Ubuntu" || run.got != "echo hello" {
		t.Errorf("command not forwarded correctly: vm=%q cmd=%q", run.vm, run.got)
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

func TestVMExecWindowsCommandForwardedVerbatim(t *testing.T) {
	tools, run := newExecTools(&parallels.CmdResult{ExitCode: 0}, nil)
	cmd := `cmd /c "echo %USERNAME% & dir C:\Users\Admin"`
	res, _, _ := tools.vmExec(context.Background(), &mcp.CallToolRequest{}, vmExecInput{
		VM: "Windows 11", Command: cmd,
	})
	if res.IsError {
		t.Fatalf("unexpected error: %s", textOf(t, res))
	}
	if run.got != cmd {
		t.Errorf("Windows command mangled:\n got  %q\n want %q", run.got, cmd)
	}
}
