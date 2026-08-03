package parallels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// Client is the high-level Parallels CLI wrapper used by the MCP tools layer.
// Methods translate domain operations into prlctl/prlsrvctl calls and parse the
// results into typed Go values. Construct with [New].
type Client struct {
	Run Runner
}

// New returns a Client backed by the real prlctl/prlsrvctl binaries.
func New() *Client { return &Client{Run: ExecRunner{}} }

// exec runs `bin args...` and returns the captured result. A non-zero exit is
// surfaced as an [*ExitError]; the partially-populated result is still returned.
func (c *Client) exec(ctx context.Context, bin string, args ...string) (*CmdResult, error) {
	if c.Run == nil {
		c.Run = ExecRunner{}
	}
	return c.Run.Run(ctx, bin, args...)
}

// runJSON runs `bin args...` and unmarshals stdout into out. Empty/whitespace
// stdout is treated as an error with a clear message (Parallels emits nothing
// on some "not found" paths).
func (c *Client) runJSON(ctx context.Context, out any, bin string, args ...string) (*CmdResult, error) {
	r, err := c.exec(ctx, bin, args...)
	if err != nil {
		return r, err
	}
	trimmed := strings.TrimSpace(r.Stdout)
	if trimmed == "" {
		return r, &ExitError{Bin: bin, Args: args, Code: r.ExitCode, Stderr: r.Stderr,
			Err: fmt.Errorf("empty output")}
	}
	if err := json.Unmarshal([]byte(r.Stdout), out); err != nil {
		return r, fmt.Errorf("parse %s output: %w", bin, err)
	}
	return r, nil
}

// ok runs `bin args...` expecting no structured output, returning only errors.
func (c *Client) ok(ctx context.Context, bin string, args ...string) error {
	_, err := c.exec(ctx, bin, args...)
	return err
}
