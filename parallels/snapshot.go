package parallels

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// SnapshotCreate creates a named snapshot of vmID, optionally described.
func (c *Client) SnapshotCreate(ctx context.Context, vmID, name, desc string) error {
	args := []string{"snapshot", vmID, "-n", name}
	if strings.TrimSpace(desc) != "" {
		args = append(args, "-d", desc)
	}
	return c.ok(ctx, Prlctl, args...)
}

// SnapshotList returns all snapshots of vmID. An empty slice means the VM has
// no snapshots.
func (c *Client) SnapshotList(ctx context.Context, vmID string) ([]Snapshot, error) {
	r, err := c.exec(ctx, Prlctl, "snapshot-list", vmID, "-j")
	if err != nil {
		return nil, err
	}
	out := strings.TrimSpace(r.Stdout)
	if out == "" || out == "[]" {
		return nil, nil
	}
	var snaps []Snapshot
	if err := json.Unmarshal([]byte(out), &snaps); err != nil {
		return nil, fmt.Errorf("parse snapshot-list output: %w", err)
	}
	return snaps, nil
}

// SnapshotRestore reverts vmID to a snapshot identified by snapID (preferred) or
// snapName. Exactly one of the two must be provided.
func (c *Client) SnapshotRestore(ctx context.Context, vmID, snapID, snapName string) error {
	args := []string{"snapshot-switch", vmID}
	switch {
	case strings.TrimSpace(snapID) != "":
		args = append(args, "-i", snapID)
	case strings.TrimSpace(snapName) != "":
		args = append(args, "-n", snapName)
	default:
		return fmt.Errorf("snapshot id or name is required")
	}
	return c.ok(ctx, Prlctl, args...)
}
