package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *Tools) registerSnapshots(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_snapshot_create",
		Description: "Create a named snapshot of a VM, with an optional description.",
	}, t.snapshotCreate)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_snapshot_list",
		Description: "List all snapshots for a VM.",
	}, t.snapshotList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_snapshot_restore",
		Description: "Restore (revert) a VM to a specific snapshot by id or name.",
	}, t.snapshotRestore)
}

type snapshotCreateInput struct {
	VM          string `json:"vm" jsonschema:"VM name or UUID"`
	Name        string `json:"name" jsonschema:"snapshot name"`
	Description string `json:"description,omitempty" jsonschema:"optional description for the snapshot"`
}

func (t *Tools) snapshotCreate(ctx context.Context, req *mcp.CallToolRequest, in snapshotCreateInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.Name == "" {
		return errResult("`vm` and `name` are required"), noOut{}, nil
	}
	if err := t.cli.SnapshotCreate(ctx, in.VM, in.Name, in.Description); err != nil {
		return fail("create snapshot", err), noOut{}, nil
	}
	msg := fmt.Sprintf("✅ Created snapshot **%s** on **%s**.", in.Name, in.VM)
	if in.Description != "" {
		msg += fmt.Sprintf("\n\n_description: %s_", in.Description)
	}
	return textResult(msg), noOut{}, nil
}

type snapshotListInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) snapshotList(ctx context.Context, req *mcp.CallToolRequest, in snapshotListInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	snaps, err := t.cli.SnapshotList(ctx, in.VM)
	if err != nil {
		return fail("list snapshots", err), noOut{}, nil
	}
	if len(snaps) == 0 {
		return textResult(fmt.Sprintf("**%s** has no snapshots.", in.VM)), noOut{}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## Snapshots of %s\n\n", in.VM)
	b.WriteString("| current | name | id | date | state | description |\n|---|---|---|---|---|---|\n")
	for _, s := range snaps {
		cur := ""
		if s.IsCurrent() {
			cur = "✅"
		}
		fmt.Fprintf(&b, "| %s | %s | `%s` | %s | %s | %s |\n",
			cur, or(s.Name, "—"), s.ID, or(s.Date, "—"), or(s.State, "—"), or(s.Description, "—"))
	}
	return textResult(b.String()), noOut{}, nil
}

type snapshotRestoreInput struct {
	VM   string `json:"vm" jsonschema:"VM name or UUID"`
	ID   string `json:"id,omitempty" jsonschema:"snapshot id to restore (preferred over name)"`
	Name string `json:"name,omitempty" jsonschema:"snapshot name to restore (used when id is omitted)"`
}

func (t *Tools) snapshotRestore(ctx context.Context, req *mcp.CallToolRequest, in snapshotRestoreInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if in.ID == "" && in.Name == "" {
		return errResult("provide snapshot `id` or `name` to restore"), noOut{}, nil
	}
	if err := t.cli.SnapshotRestore(ctx, in.VM, in.ID, in.Name); err != nil {
		return fail("restore snapshot", err), noOut{}, nil
	}
	target := in.ID
	if target == "" {
		target = in.Name
	}
	return textResult(fmt.Sprintf("✅ Restored **%s** to snapshot **%s**.", in.VM, target)), noOut{}, nil
}
