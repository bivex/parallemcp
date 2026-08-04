package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerSharedFolders(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_shared_folders",
		Description: "Manage host shared folders for a VM (list, add, remove).",
	}, t.vmSharedFolders)
}

type vmSharedFoldersInput struct {
	Action string `json:"action" jsonschema:"enum=list,enum=add,enum=remove,action to perform: list, add, or remove"`
	VM     string `json:"vm" jsonschema:"VM name or UUID"`
	Name   string `json:"name,omitempty" jsonschema:"shared folder name (required for add and remove)"`
	Path   string `json:"path,omitempty" jsonschema:"host path (required for add)"`
	Mode   string `json:"mode,omitempty" jsonschema:"enum=rw,enum=ro,access mode for add (rw or ro, default rw)"`
}

func (t *Tools) vmSharedFolders(ctx context.Context, req *mcp.CallToolRequest, in vmSharedFoldersInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}

	switch strings.ToLower(in.Action) {
	case "list":
		info, err := t.cli.Info(ctx, in.VM)
		if err != nil {
			return fail("list shared folders", err), noOut{}, nil
		}
		sfs := info.SharedFolders()
		if len(sfs) == 0 {
			return textResult(fmt.Sprintf("No host shared folders configured for **%s**.", in.VM)), noOut{}, nil
		}
		var b strings.Builder
		fmt.Fprintf(&b, "## Host Shared Folders @ %s\n\n", in.VM)
		b.WriteString("| Name | Host Path | Mode | Enabled |\n|---|---|---|---|\n")
		for _, sf := range sfs {
			mode := sf.Mode
			if mode == "" {
				mode = "rw"
			}
			enabled := "yes"
			if !sf.Enabled {
				enabled = "no"
			}
			fmt.Fprintf(&b, "| %s | `%s` | %s | %s |\n", sf.Name, sf.Path, mode, enabled)
		}
		return textResult(b.String()), noOut{}, nil

	case "add":
		if in.Name == "" || in.Path == "" {
			return errResult("`name` and `path` are required when action is 'add'"), noOut{}, nil
		}
		mode := in.Mode
		if mode == "" {
			mode = "rw"
		}
		err := t.cli.SharedFolderAdd(ctx, in.VM, parallels.SharedFolderAddParams{
			Name: in.Name,
			Path: in.Path,
			Mode: mode,
		})
		if err != nil {
			return fail("add shared folder", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("✅ Added shared folder **%s** (`%s`, mode: %s) to **%s**.", in.Name, in.Path, mode, in.VM)), noOut{}, nil

	case "remove", "del", "delete":
		if in.Name == "" {
			return errResult("`name` is required when action is 'remove'"), noOut{}, nil
		}
		err := t.cli.SharedFolderRemove(ctx, in.VM, in.Name)
		if err != nil {
			return fail("remove shared folder", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("✅ Removed shared folder **%s** from **%s**.", in.Name, in.VM)), noOut{}, nil

	default:
		return errResult("`action` must be one of: list, add, remove"), noOut{}, nil
	}
}
