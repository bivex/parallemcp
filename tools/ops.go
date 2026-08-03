package tools

import (
	"context"
	"fmt"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerOps(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_exec",
		Description: "Execute a shell command inside a running VM (requires Parallels Tools). Runs the command with /bin/sh -lc, so pipes, redirection and && work.",
	}, t.vmExec)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_configure",
		Description: "Change a VM's CPU count, memory (MB), and/or name. Only the fields you provide are applied.",
	}, t.vmConfigure)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "server_info",
		Description: "Parallels Desktop version, license state, host OS, and VM home directory.",
	}, t.serverInfo)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "host_stats",
		Description: "macOS host statistics: chip, CPU cores, memory (total/free), root disk usage, load average, and uptime.",
	}, t.hostStats)
}

type vmExecInput struct {
	VM      string `json:"vm" jsonschema:"VM name or UUID"`
	Command string `json:"command" jsonschema:"shell command to run inside the VM"`
}

func (t *Tools) vmExec(ctx context.Context, req *mcp.CallToolRequest, in vmExecInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || strings.TrimSpace(in.Command) == "" {
		return errResult("`vm` and `command` are required"), noOut{}, nil
	}
	r, err := t.cli.Exec(ctx, in.VM, in.Command)
	if err != nil {
		return fail("exec command", err), noOut{}, nil
	}
	var b strings.Builder
	fmt.Fprintf(&b, "## `%s` @ %s (exit %d)\n\n", in.Command, in.VM, r.ExitCode)
	if r.Stdout != "" {
		fmt.Fprintf(&b, "**stdout:**\n```\n%s\n```\n", strings.TrimRight(r.Stdout, "\n"))
	}
	if r.Stderr != "" {
		fmt.Fprintf(&b, "**stderr:**\n```\n%s\n```\n", strings.TrimRight(r.Stderr, "\n"))
	}
	if r.Stdout == "" && r.Stderr == "" {
		b.WriteString("_(no output)_\n")
	}
	return textResult(b.String()), noOut{}, nil
}

type vmConfigureInput struct {
	VM       string `json:"vm" jsonschema:"VM name or UUID"`
	CPUs     int    `json:"cpus,omitempty" jsonschema:"new vCPU count"`
	MemoryMB int    `json:"memory_mb,omitempty" jsonschema:"new memory in megabytes"`
	Name     string `json:"name,omitempty" jsonschema:"new VM name"`
}

func (t *Tools) vmConfigure(ctx context.Context, req *mcp.CallToolRequest, in vmConfigureInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Configure(ctx, in.VM, parallels.ConfigureParams{
		CPUs:     in.CPUs,
		MemoryMB: in.MemoryMB,
		Name:     in.Name,
	}); err != nil {
		return fail("configure VM", err), noOut{}, nil
	}
	var changes []string
	if in.CPUs > 0 {
		changes = append(changes, fmt.Sprintf("cpus=%d", in.CPUs))
	}
	if in.MemoryMB > 0 {
		changes = append(changes, fmt.Sprintf("memory=%dMB", in.MemoryMB))
	}
	if in.Name != "" {
		changes = append(changes, "name="+in.Name)
	}
	return textResult(fmt.Sprintf("✅ Configured **%s**: %s.\n\n_Memory changes apply on the next VM start._",
		in.VM, strings.Join(changes, ", "))), noOut{}, nil
}

type serverInfoInput struct{}

func (t *Tools) serverInfo(ctx context.Context, req *mcp.CallToolRequest, in serverInfoInput) (*mcp.CallToolResult, noOut, error) {
	si, err := t.cli.ServerInfo(ctx)
	if err != nil {
		return fail("read server info", err), noOut{}, nil
	}
	var b strings.Builder
	b.WriteString("## Parallels Desktop\n\n")
	fmt.Fprintf(&b, "- **version**: %s\n", or(si.Version, "—"))
	fmt.Fprintf(&b, "- **license**: %s", or(si.License.State, "—"))
	if si.License.Restricted != "" {
		fmt.Fprintf(&b, " (restricted=%s)", si.License.Restricted)
	}
	b.WriteString("\n")
	fmt.Fprintf(&b, "- **host OS**: %s\n", or(si.OS, "—"))
	fmt.Fprintf(&b, "- **hostname**: %s\n", or(si.Hostname, "—"))
	fmt.Fprintf(&b, "- **vm home**: `%s`\n", or(si.VMHome, "—"))
	fmt.Fprintf(&b, "- **server id**: `%s`\n", si.ID)
	return textResult(b.String()), noOut{}, nil
}

type hostStatsInput struct{}

func (t *Tools) hostStats(ctx context.Context, req *mcp.CallToolRequest, in hostStatsInput) (*mcp.CallToolResult, noOut, error) {
	hs, err := t.cli.HostStats(ctx)
	if err != nil {
		return fail("read host stats", err), noOut{}, nil
	}
	var b strings.Builder
	b.WriteString("## macOS host\n\n")
	fmt.Fprintf(&b, "- **chip**: %s\n", or(hs.Chip, "—"))
	fmt.Fprintf(&b, "- **cores**: %d physical / %d logical\n", hs.PhysicalCores, hs.LogicalCores)
	fmt.Fprintf(&b, "- **memory**: %.1f GB total / %.1f GB free\n", hs.MemoryTotalGB, hs.MemoryFreeGB)
	if hs.DiskTotalGB > 0 {
		pct := 0.0
		if hs.DiskTotalGB > 0 {
			pct = 100 * hs.DiskUsedGB / hs.DiskTotalGB
		}
		fmt.Fprintf(&b, "- **root disk**: %.0f / %.0f GB used (%.0f%%)\n", hs.DiskUsedGB, hs.DiskTotalGB, pct)
	}
	fmt.Fprintf(&b, "- **load avg**: %.2f, %.2f, %.2f (1/5/15m)\n", hs.LoadAvg1, hs.LoadAvg5, hs.LoadAvg15)
	fmt.Fprintf(&b, "- **uptime**: %.1f days\n", hs.UptimeDays)
	return textResult(b.String()), noOut{}, nil
}
