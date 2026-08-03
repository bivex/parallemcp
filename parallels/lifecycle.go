package parallels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// List returns all registered VMs (running, stopped, suspended, ...).
func (c *Client) List(ctx context.Context) ([]VMListEntry, error) {
	var out []VMListEntry
	if _, err := c.runJSON(ctx, &out, Prlctl, "list", "-a", "--json"); err != nil {
		return nil, err
	}
	return out, nil
}

// Find returns the list entry whose name or UUID matches id. It is the shared
// resolver behind vm_status and other per-VM tools.
func (c *Client) Find(ctx context.Context, id string) (VMListEntry, error) {
	all, err := c.List(ctx)
	if err != nil {
		return VMListEntry{}, err
	}
	for _, e := range all {
		if e.Name == id || strings.EqualFold(e.UUID, id) {
			return e, nil
		}
	}
	return VMListEntry{}, &ExitError{
		Bin: Prlctl, Args: []string{"status", id},
		Err: fmt.Errorf("VM %q not found", id),
	}
}

// Status returns the list entry (name, uuid, status, configured IP) for a VM.
func (c *Client) Status(ctx context.Context, id string) (VMListEntry, error) {
	return c.Find(ctx, id)
}

// Info returns the full configuration for a VM. The CLI emits a JSON array; we
// also tolerate a bare object for forward/backward compatibility.
func (c *Client) Info(ctx context.Context, id string) (*VMInfo, error) {
	r, err := c.exec(ctx, Prlctl, "list", "-i", id, "--json")
	if err != nil {
		return nil, err
	}
	out := strings.TrimSpace(r.Stdout)
	if out == "" {
		return nil, fmt.Errorf("no info returned for %q", id)
	}
	if arr := tryArrVMInfo(out); len(arr) > 0 {
		return &arr[0], nil
	}
	if single, ok := tryOneVMInfo(out); ok {
		return single, nil
	}
	return nil, fmt.Errorf("could not parse info for %q", id)
}

func tryArrVMInfo(s string) []VMInfo {
	var arr []VMInfo
	if err := json.Unmarshal([]byte(s), &arr); err != nil {
		return nil
	}
	return arr
}

func tryOneVMInfo(s string) (*VMInfo, bool) {
	var v VMInfo
	if err := json.Unmarshal([]byte(s), &v); err != nil || v.ID == "" {
		return nil, false
	}
	return &v, true
}

// Start boots a stopped or suspended VM.
func (c *Client) Start(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "start", id)
}

// Stop shuts a VM down. With force it kills the process immediately (--kill).
func (c *Client) Stop(ctx context.Context, id string, force bool) error {
	args := []string{"stop", id}
	if force {
		args = append(args, "--kill")
	}
	return c.ok(ctx, Prlctl, args...)
}

// Restart reboots a running VM.
func (c *Client) Restart(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "restart", id)
}

// Suspend saves VM state to disk (pause-to-disk).
func (c *Client) Suspend(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "suspend", id)
}

// Resume wakes a suspended VM.
func (c *Client) Resume(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "resume", id)
}
