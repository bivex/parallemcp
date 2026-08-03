package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func (t *Tools) registerLifecycle(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_list",
		Description: "List all VMs with their status and configured IP address.",
	}, t.vmList)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_status",
		Description: "Get the status of a specific VM (by name or UUID).",
	}, t.vmStatus)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_info",
		Description: "Full VM configuration: CPU, memory, disks, network adapters, and guest IP addresses.",
	}, t.vmInfo)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_start",
		Description: "Start a stopped or suspended VM.",
	}, t.vmStart)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_stop",
		Description: "Stop a VM. Graceful shutdown by default; set force=true to kill it immediately.",
	}, t.vmStop)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_restart",
		Description: "Restart a running VM.",
	}, t.vmRestart)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_suspend",
		Description: "Suspend a VM (save its state to disk).",
	}, t.vmSuspend)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_resume",
		Description: "Resume a suspended VM.",
	}, t.vmResume)
}

type vmListInput struct{}

func (t *Tools) vmList(ctx context.Context, req *mcp.CallToolRequest, in vmListInput) (*mcp.CallToolResult, noOut, error) {
	vms, err := t.cli.List(ctx)
	if err != nil {
		return fail("list VMs", err), noOut{}, nil
	}
	if len(vms) == 0 {
		return textResult("_No VMs are registered._"), noOut{}, nil
	}
	var b strings.Builder
	b.WriteString("| Name | Status | IP | UUID |\n|---|---|---|---|\n")
	for _, v := range vms {
		fmt.Fprintf(&b, "| %s | %s | %s | `%s` |\n", v.Name, v.Status, ipDisp(v.IPConfigured), v.UUID)
	}
	return textResult(b.String()), noOut{}, nil
}

type vmRefInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmStatus(ctx context.Context, req *mcp.CallToolRequest, in vmRefInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	v, err := t.cli.Status(ctx, in.VM)
	if err != nil {
		return fail("get status", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf(
		"**%s**\n\n- status: `%s`\n- ip_configured: `%s`\n- uuid: `%s`",
		v.Name, v.Status, ipDisp(v.IPConfigured), v.UUID,
	)), noOut{}, nil
}

func (t *Tools) vmInfo(ctx context.Context, req *mcp.CallToolRequest, in vmRefInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	info, err := t.cli.Info(ctx, in.VM)
	if err != nil {
		return fail("get VM info", err), noOut{}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## %s\n\n", info.Name)
	fmt.Fprintf(&b, "- **state**: %s\n", info.State)
	fmt.Fprintf(&b, "- **os**: %s\n", info.OS)
	fmt.Fprintf(&b, "- **uuid**: `%s`\n", info.ID)
	if info.HomePath != "" {
		fmt.Fprintf(&b, "- **home**: `%s`\n", info.HomePath)
	}
	if info.Uptime != "" {
		fmt.Fprintf(&b, "- **uptime**: %ss\n", info.Uptime)
	}
	if info.GuestTools.State != "" {
		fmt.Fprintf(&b, "- **guest tools**: %s", info.GuestTools.State)
		if info.GuestTools.Version != "" {
			fmt.Fprintf(&b, " (%s)", info.GuestTools.Version)
		}
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "- **cpu**: %d vCPU\n", info.CPUs())
	if mb := info.MemoryMB(); mb > 0 {
		fmt.Fprintf(&b, "- **memory**: %d MB (%.1f GB)\n", mb, float64(mb)/1024)
	}

	if disks := info.Disks(); len(disks) > 0 {
		b.WriteString("\n### Disks\n\n| type | size | enabled | image |\n|---|---|---|---|\n")
		for _, d := range disks {
			fmt.Fprintf(&b, "| %s | %s | %v | `%s` |\n", or(d.Type, "—"), or(d.Size, "—"), d.Enabled, shortPath(d.Image))
		}
	}
	if nets := info.Nets(); len(nets) > 0 {
		b.WriteString("\n### Network\n\n| type | card | iface | mac | enabled |\n|---|---|---|---|---|\n")
		for _, n := range nets {
			fmt.Fprintf(&b, "| %s | %s | %s | `%s` | %v |\n",
				or(n.Type, "—"), or(n.Card, "—"), or(n.Iface, "—"), n.MAC, n.Enabled)
		}
	}
	if ips := info.IPv4s(); len(ips) > 0 {
		b.WriteString("\n### Guest IPv4 addresses\n\n")
		for _, ip := range ips {
			fmt.Fprintf(&b, "- `%s`\n", ip)
		}
	}
	return textResult(b.String()), noOut{}, nil
}

type vmStartInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmStart(ctx context.Context, req *mcp.CallToolRequest, in vmStartInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Start(ctx, in.VM); err != nil {
		return fail("start VM", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("✅ Started **%s**.", in.VM)), noOut{}, nil
}

type vmStopInput struct {
	VM    string `json:"vm" jsonschema:"VM name or UUID"`
	Force bool   `json:"force,omitempty" jsonschema:"force-kill the VM immediately instead of a graceful shutdown"`
}

func (t *Tools) vmStop(ctx context.Context, req *mcp.CallToolRequest, in vmStopInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Stop(ctx, in.VM, in.Force); err != nil {
		return fail("stop VM", err), noOut{}, nil
	}
	how := "gracefully stopped"
	if in.Force {
		how = "force-killed"
	}
	return textResult(fmt.Sprintf("✅ %s **%s**.", how, in.VM)), noOut{}, nil
}

type vmRestartInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmRestart(ctx context.Context, req *mcp.CallToolRequest, in vmRestartInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Restart(ctx, in.VM); err != nil {
		return fail("restart VM", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("✅ Restarted **%s**.", in.VM)), noOut{}, nil
}

type vmSuspendInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmSuspend(ctx context.Context, req *mcp.CallToolRequest, in vmSuspendInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Suspend(ctx, in.VM); err != nil {
		return fail("suspend VM", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("✅ Suspended **%s**.", in.VM)), noOut{}, nil
}

type vmResumeInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmResume(ctx context.Context, req *mcp.CallToolRequest, in vmResumeInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Resume(ctx, in.VM); err != nil {
		return fail("resume VM", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("✅ Resumed **%s**.", in.VM)), noOut{}, nil
}

// or returns s, falling back to alt when s is empty.
func or(s, alt string) string {
	if strings.TrimSpace(s) == "" {
		return alt
	}
	return s
}

// shortPath keeps disk image paths readable in tables.
func shortPath(p string) string {
	if i := strings.LastIndex(p, "/"); i >= 0 {
		return p[i+1:]
	}
	return p
}
