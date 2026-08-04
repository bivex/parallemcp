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
	if out == "" || out == "[]" || out == "{}" {
		return nil, nil
	}
	var snaps []Snapshot
	if err := json.Unmarshal([]byte(out), &snaps); err == nil {
		return snaps, nil
	}

	var snapMap map[string]struct {
		Name        string      `json:"name"`
		Date        string      `json:"date"`
		Current     interface{} `json:"current"`
		State       string      `json:"state"`
		Description string      `json:"description"`
		Parent      string      `json:"parent"`
	}
	if err := json.Unmarshal([]byte(out), &snapMap); err != nil {
		return nil, fmt.Errorf("parse snapshot-list output: %w", err)
	}

	for id, s := range snapMap {
		currStr := fmt.Sprintf("%v", s.Current)
		snaps = append(snaps, Snapshot{
			ID:          id,
			Name:        s.Name,
			Date:        s.Date,
			Current:     currStr,
			State:       s.State,
			Description: s.Description,
			Parent:      s.Parent,
		})
	}
	return snaps, nil
}

// SnapshotRestore reverts vmID to a snapshot identified by snapID (preferred) or
// snapName. Exactly one of the two must be provided.
func (c *Client) SnapshotRestore(ctx context.Context, vmID, snapID, snapName string) error {
	id := strings.TrimSpace(snapID)
	if id == "" {
		id = strings.TrimSpace(snapName)
	}
	if id == "" {
		return fmt.Errorf("snapshot id or name is required")
	}
	return c.ok(ctx, Prlctl, "snapshot-switch", vmID, "-i", id)
}

// SnapshotDelete deletes a snapshot identified by snapID or snapName.
func (c *Client) SnapshotDelete(ctx context.Context, vmID, snapID string) error {
	if strings.TrimSpace(snapID) == "" {
		return fmt.Errorf("snapshot id or name is required")
	}
	return c.ok(ctx, Prlctl, "snapshot-delete", vmID, "-i", snapID)
}
