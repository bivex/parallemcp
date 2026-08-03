package parallels

import (
	"context"
	"errors"
	"testing"
)

// keyFor returns the stub key used by stubRunner for a prlctl exec call.
func execKey(id, command string) string {
	return "prlctl exec " + id + " /bin/sh -lc " + command
}

func TestExecSuccess(t *testing.T) {
	c := &Client{Run: newStub(map[string]string{
		execKey("vm", "echo hi"): "hi\n",
	})}
	r, err := c.Exec(context.Background(), "vm", "echo hi")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if r.ExitCode != 0 {
		t.Errorf("ExitCode = %d, want 0", r.ExitCode)
	}
	if r.Stdout != "hi\n" {
		t.Errorf("Stdout = %q, want %q", r.Stdout, "hi\n")
	}
}

func TestExecGuestNonZeroExitIsNotError(t *testing.T) {
	// By design the CLI runner returns an error on ANY non-zero exit, so we feed
	// an ExitError. The Exec wrapper swallows it (returns no Go error) unless the
	// stderr looks like a failure to *run* the command — the caller decides how
	// to present a guest non-zero exit via ExitCode.
	stub := &stubRunner{responses: map[string]stubResp{
		execKey("vm", "false"): {
			out: &CmdResult{Stderr: "boom", ExitCode: 1},
			err: &ExitError{Code: 1, Stderr: "boom"},
		},
	}}
	c := &Client{Run: stub}
	r, err := c.Exec(context.Background(), "vm", "false")
	if err != nil {
		t.Fatalf("expected no error for guest non-zero exit, got %v", err)
	}
	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}
	if r.Stderr != "boom" {
		t.Errorf("Stderr = %q, want %q", r.Stderr, "boom")
	}
}

func TestExecGuestFailureReturnsError(t *testing.T) {
	// When prlctl cannot run the command at all (e.g. Parallels Tools missing),
	// the error must be returned so the tool can flag it.
	stub := &stubRunner{responses: map[string]stubResp{
		execKey("vm", "echo hi"): {
			out: &CmdResult{Stderr: "Unable to execute: Parallels Tools are not installed", ExitCode: 1},
			err: &ExitError{
				Code:   1,
				Stderr: "Unable to execute: Parallels Tools are not installed",
			},
		},
	}}
	c := &Client{Run: stub}
	r, err := c.Exec(context.Background(), "vm", "echo hi")
	if err == nil {
		t.Fatal("expected error for exec failure")
	}
	if r.ExitCode != 1 {
		t.Errorf("ExitCode = %d, want 1", r.ExitCode)
	}
}

func TestExecContextCanceled(t *testing.T) {
	stub := &stubRunner{responses: map[string]stubResp{
		execKey("vm", "sleep 100"): {out: &CmdResult{}, err: context.Canceled},
	}}
	c := &Client{Run: stub}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Exec(ctx, "vm", "sleep 100"); !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
}

func TestExecEmptyOutput(t *testing.T) {
	stub := newStub(map[string]string{
		execKey("vm", "true"): "", // exit 0, nothing on stdout/stderr
	})
	c := &Client{Run: stub}
	r, err := c.Exec(context.Background(), "vm", "true")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if r.Stdout != "" || r.Stderr != "" || r.ExitCode != 0 {
		t.Errorf("unexpected result: %+v", r)
	}
}

func TestExecMultilineAndTrailingNewlinePreserved(t *testing.T) {
	stub := newStub(map[string]string{
		execKey("vm", "printf a"): "line1\nline2\nline3\n",
	})
	c := &Client{Run: stub}
	r, err := c.Exec(context.Background(), "vm", "printf a")
	if err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if r.Stdout != "line1\nline2\nline3\n" {
		t.Errorf("Stdout = %q, want verbatim multiline", r.Stdout)
	}
}

// TestExecCommandEscapingLinux verifies the command reaches the guest shell
// verbatim as a single argument — the Go wrapper must NOT re-split, re-quote, or
// interpret any shell metacharacters. Everything (quotes, $vars, backticks,
// pipes, redirect, &&, ;, newlines) is handed intact to /bin/sh -lc.
func TestExecCommandEscapingLinux(t *testing.T) {
	var gotArgs []string
	c := &Client{Run: &captureRunner{on: func(_ string, args []string) {
		gotArgs = append([]string(nil), args...)
	}}}

	cmd := `echo "a;b" && cat /etc/passwd | grep root; echo $HOME ` +
		"`whoami`" +
		` > /tmp/x <<'EOF'
heredoc line
EOF`

	if _, err := c.Exec(context.Background(), "linux-vm", cmd); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	// prlctl exec <vm> /bin/sh -lc <command> ⇒ 5 args.
	if len(gotArgs) != 5 {
		t.Fatalf("arg count = %d, want 5: %v", len(gotArgs), gotArgs)
	}
	if gotArgs[0] != "exec" || gotArgs[1] != "linux-vm" {
		t.Errorf("unexpected prefix: %v", gotArgs)
	}
	if gotArgs[2] != "/bin/sh" || gotArgs[3] != "-lc" {
		t.Errorf("expected /bin/sh -lc wrapper, got %v", gotArgs[2:4])
	}
	if gotArgs[4] != cmd {
		t.Errorf("command not passed verbatim:\n got  %q\n want %q", gotArgs[4], cmd)
	}
}

// TestExecCommandEscapingWindows verifies a Windows-style command (backslashes,
// %VAR% expansion, & chaining, ^ escaping, quotes) is preserved verbatim by the
// Go wrapper. prlctl routes the command to the guest shell appropriate for the
// OS; we only assert the string is never mangled client-side.
func TestExecCommandEscapingWindows(t *testing.T) {
	var gotArgs []string
	c := &Client{Run: &captureRunner{on: func(_ string, args []string) {
		gotArgs = append([]string(nil), args...)
	}}}

	cmd := `cmd /c "echo %USERNAME% & dir C:\Users\Admin & type C:\tmp\log.txt"`

	if _, err := c.Exec(context.Background(), "win-vm", cmd); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if len(gotArgs) != 5 {
		t.Fatalf("arg count = %d, want 5: %v", len(gotArgs), gotArgs)
	}
	if gotArgs[2] != "/bin/sh" || gotArgs[3] != "-lc" {
		t.Errorf("expected /bin/sh -lc wrapper, got %v", gotArgs[2:4])
	}
	if gotArgs[4] != cmd {
		t.Errorf("command not passed verbatim:\n got  %q\n want %q", gotArgs[4], cmd)
	}
}

// TestExecCommandWithSpacesAndUnicode ensures whitespace-heavy and non-ASCII
// commands survive round-trip intact (no trimming, no encoding corruption).
func TestExecCommandWithSpacesAndUnicode(t *testing.T) {
	var got string
	c := &Client{Run: &captureRunner{on: func(_ string, args []string) {
		got = args[len(args)-1]
	}}}
	cmd := "echo '  leading and trailing  ' && printf 'café — 日本語'"
	if _, err := c.Exec(context.Background(), "vm", cmd); err != nil {
		t.Fatalf("Exec: %v", err)
	}
	if got != cmd {
		t.Errorf("command mangled:\n got  %q\n want %q", got, cmd)
	}
}

// TestLooksLikeExecFailure locks in the substrings that distinguish a failure to
// *run* the command (error) from a guest command merely exiting non-zero.
func TestLooksLikeExecFailure(t *testing.T) {
	cases := []struct {
		in   string
		want bool
	}{
		{"Unable to execute the command", true},
		{"Unable to perform action", true},
		{"Parallels Tools are not installed", true},
		{"The VM is not running", true},
		{"Could not be found", true},
		{"not registered", true},
		{"a guest program that prints 'parallels tools' casually", true}, // substring match
		{"regular command error: No such file or directory", false},
		{"permission denied", false},
		{"", false},
	}
	for _, tc := range cases {
		if got := looksLikeExecFailure(tc.in); got != tc.want {
			t.Errorf("looksLikeExecFailure(%q) = %v, want %v", tc.in, got, tc.want)
		}
	}
}
