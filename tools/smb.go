package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerSMB(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_smb",
		Description: "Manage Windows SMB network shares and mapped network drives between VMs (list, share, mount, unmount).",
	}, t.vmSMB)
}

type vmSMBInput struct {
	Action      string `json:"action" jsonschema:"action to perform: 'list', 'share', 'mount', or 'unmount'"`
	VM          string `json:"vm" jsonschema:"VM name or UUID"`
	ShareName   string `json:"share_name,omitempty" jsonschema:"name of the SMB share"`
	FolderPath  string `json:"folder_path,omitempty" jsonschema:"local folder path for 'share' action (e.g. 'C:\\NetworkShare')"`
	RemoteIP    string `json:"remote_ip,omitempty" jsonschema:"remote VM IP address for 'mount' action (e.g. '10.211.55.4')"`
	DriveLetter string `json:"drive_letter,omitempty" jsonschema:"drive letter for 'mount' or 'unmount' (default 'S:')"`
}

func (t *Tools) vmSMB(ctx context.Context, req *mcp.CallToolRequest, in vmSMBInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}

	switch strings.ToLower(in.Action) {
	case "list":
		info, err := t.cli.Info(ctx, in.VM)
		if err != nil {
			return fail("read vm info", err), noOut{}, nil
		}
		resShare, _ := t.cli.ExecOS(ctx, in.VM, "net share", info.OS)
		resUse, _ := t.cli.ExecOS(ctx, in.VM, "net use", info.OS)

		var b strings.Builder
		fmt.Fprintf(&b, "## SMB Network Configuration @ %s\n\n", in.VM)
		b.WriteString("### Local SMB Shares (`net share`)\n```\n")
		if resShare != nil && resShare.Stdout != "" {
			b.WriteString(strings.TrimSpace(resShare.Stdout))
		} else {
			b.WriteString("(none)")
		}
		b.WriteString("\n```\n\n### Mapped Network Drives (`net use`)\n```\n")
		if resUse != nil && resUse.Stdout != "" {
			b.WriteString(strings.TrimSpace(resUse.Stdout))
		} else {
			b.WriteString("(none)")
		}
		b.WriteString("\n```\n")
		return textResult(b.String()), noOut{}, nil

	case "share", "create":
		if in.ShareName == "" || in.FolderPath == "" {
			return errResult("`share_name` and `folder_path` are required when action is 'share'"), noOut{}, nil
		}
		err := t.cli.SMBShare(ctx, in.VM, parallels.SMBShareParams{
			ShareName:  in.ShareName,
			FolderPath: in.FolderPath,
		})
		if err != nil {
			return fail("create smb share", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("✅ Shared folder **%s** as SMB share **\\\\%s\\%s**.", in.FolderPath, in.VM, in.ShareName)), noOut{}, nil

	case "mount", "connect":
		if in.RemoteIP == "" || in.ShareName == "" {
			return errResult("`remote_ip` and `share_name` are required when action is 'mount'"), noOut{}, nil
		}
		drive := in.DriveLetter
		if drive == "" {
			drive = "S:"
		}
		err := t.cli.SMBMount(ctx, in.VM, parallels.SMBMountParams{
			RemoteIP:    in.RemoteIP,
			ShareName:   in.ShareName,
			DriveLetter: drive,
		})
		if err != nil {
			return fail("mount smb share", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("✅ Mounted **\\\\%s\\%s** to drive **%s** on **%s**.", in.RemoteIP, in.ShareName, drive, in.VM)), noOut{}, nil

	case "unmount", "disconnect", "delete":
		err := t.cli.SMBUnmount(ctx, in.VM, parallels.SMBUnmountParams{
			DriveLetter: in.DriveLetter,
			ShareName:   in.ShareName,
		})
		if err != nil {
			return fail("unmount smb share", err), noOut{}, nil
		}
		target := in.DriveLetter
		if target == "" {
			target = in.ShareName
		}
		return textResult(fmt.Sprintf("✅ Removed / unmounted **%s** from **%s**.", target, in.VM)), noOut{}, nil

	default:
		return errResult("`action` must be one of: list, share, mount, unmount"), noOut{}, nil
	}
}
