package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerFileCopy(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_file_copy",
		Description: "Transfer files between macOS host and guest VM.",
	}, t.vmFileCopy)
}

type vmFileCopyInput struct {
	VM        string `json:"vm" jsonschema:"VM name or UUID"`
	Direction string `json:"direction" jsonschema:"direction of transfer: 'to_guest' or 'from_guest'"`
	HostPath  string `json:"host_path" jsonschema:"absolute path on the host"`
	GuestPath string `json:"guest_path" jsonschema:"absolute path inside the guest VM"`
}

func (t *Tools) vmFileCopy(ctx context.Context, req *mcp.CallToolRequest, in vmFileCopyInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.HostPath == "" || in.GuestPath == "" {
		return errResult("`vm`, `host_path`, and `guest_path` are required"), noOut{}, nil
	}
	dir := strings.ToLower(in.Direction)
	if dir != "to_guest" && dir != "from_guest" {
		return errResult("`direction` must be 'to_guest' or 'from_guest'"), noOut{}, nil
	}

	err := t.cli.FileCopy(ctx, in.VM, parallels.FileCopyParams{
		Direction: dir,
		HostPath:  in.HostPath,
		GuestPath: in.GuestPath,
	})
	if err != nil {
		return fail("file transfer", err), noOut{}, nil
	}

	if dir == "to_guest" {
		return textResult(fmt.Sprintf("✅ Transferred **%s** (host) ➔ **%s** (%s).", in.HostPath, in.GuestPath, in.VM)), noOut{}, nil
	}
	return textResult(fmt.Sprintf("✅ Transferred **%s** (%s) ➔ **%s** (host).", in.GuestPath, in.VM, in.HostPath)), noOut{}, nil
}
