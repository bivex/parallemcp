// Package parallels is a thin Go wrapper around the Parallels Desktop CLI
// (prlctl / prlsrvctl). It contains no MCP dependencies and is fully unit
// testable by swapping in a fake [Runner].
package parallels

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
)

// Binary names, resolved through PATH at runtime.
const (
	Prlctl    = "prlctl"
	Prlsrvctl = "prlsrvctl"
)

// CmdResult holds the captured output of a Parallels CLI invocation.
type CmdResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
}

// Runner executes Parallels CLI commands. Replaced in tests with a fake that
// returns canned output, keeping the parsing logic in this package pure.
type Runner interface {
	Run(ctx context.Context, bin string, args ...string) (*CmdResult, error)
}

// ExecRunner is the production Runner: it shells out to the real binaries.
// All output is captured (stdout/stderr are never inherited) so that stray
// bytes never reach the MCP stdio transport.
type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, bin string, args ...string) (*CmdResult, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	r := &CmdResult{Stdout: stdout.String(), Stderr: stderr.String()}
	if cmd.ProcessState != nil {
		r.ExitCode = cmd.ProcessState.ExitCode()
	}
	if err == nil {
		return r, nil
	}
	if ctx.Err() != nil {
		return r, ctx.Err()
	}
	// Non-zero exit: surface stderr to the caller via a typed error.
	return r, &ExitError{Bin: bin, Args: args, Code: r.ExitCode, Stderr: r.Stderr, Err: err}
}

// ExitError is returned when the Parallels CLI exits non-zero.
type ExitError struct {
	Bin    string
	Args   []string
	Code   int
	Stderr string
	Err    error
}

func (e *ExitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" && e.Err != nil {
		msg = e.Err.Error()
	}
	return fmt.Sprintf("%s %s failed (exit %d): %s", e.Bin, quoteArgs(e.Args), e.Code, oneLine(msg))
}

func (e *ExitError) Unwrap() error { return e.Err }

// NotFound reports whether the CLI indicated the VM is unknown/unregistered.
// prlctl reports this both as text and a non-zero exit; we match common phrasings.
func (e *ExitError) NotFound() bool {
	s := strings.ToLower(e.Stderr)
	return strings.Contains(s, "could not be found") ||
		strings.Contains(s, "not registered") ||
		strings.Contains(s, "does not exist")
}

func quoteArgs(a []string) string {
	parts := make([]string, len(a))
	for i, s := range a {
		if strings.ContainsAny(s, " \t\"'") {
			parts[i] = fmt.Sprintf("%q", s)
		} else {
			parts[i] = s
		}
	}
	return strings.Join(parts, " ")
}

func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	return s
}
