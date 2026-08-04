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

	// — Core —
	fmt.Fprintf(&b, "- **state**: %s\n", info.State)
	fmt.Fprintf(&b, "- **os**: %s\n", info.OS)
	fmt.Fprintf(&b, "- **uuid**: `%s`\n", info.ID)
	if info.Description != "" {
		fmt.Fprintf(&b, "- **description**: %s\n", info.Description)
	}
	if info.HomePath != "" {
		fmt.Fprintf(&b, "- **home**: `%s`\n", info.HomePath)
	}
	if info.Uptime != "" {
		fmt.Fprintf(&b, "- **uptime**: %ss\n", info.Uptime)
	}

	// — Platform / Firmware —
	if info.BIOSType != "" || info.Optimization.HypervisorType != "" {
		b.WriteString("\n### Platform\n\n")
		if info.BIOSType != "" {
			fmt.Fprintf(&b, "- **bios**: %s\n", info.BIOSType)
		}
		if info.EFISecureBoot != "" {
			fmt.Fprintf(&b, "- **efi secure boot**: %s\n", info.EFISecureBoot)
		}
		if info.Optimization.HypervisorType != "" {
			fmt.Fprintf(&b, "- **hypervisor**: %s\n", info.Optimization.HypervisorType)
		}
		if info.Optimization.AdaptiveHV != "" {
			fmt.Fprintf(&b, "- **adaptive hypervisor**: %s\n", info.Optimization.AdaptiveHV)
		}
		if info.Optimization.FasterVM != "" {
			fmt.Fprintf(&b, "- **faster vm**: %s\n", info.Optimization.FasterVM)
		}
		if info.Optimization.NestedVirt != "" {
			fmt.Fprintf(&b, "- **nested virt**: %s\n", info.Optimization.NestedVirt)
		}
		if info.Optimization.PMUVirt != "" {
			fmt.Fprintf(&b, "- **pmu virt**: %s\n", info.Optimization.PMUVirt)
		}
		if info.Optimization.ResourceQuota != "" {
			fmt.Fprintf(&b, "- **resource quota**: %s\n", info.Optimization.ResourceQuota)
		}
		if info.BootOrder != "" {
			fmt.Fprintf(&b, "- **boot order**: `%s`\n", strings.TrimSpace(info.BootOrder))
		}
		// SMBIOS
		s := info.SMBIOS
		if s.BIOSVersion != "" || s.SerialNumber != "" || s.Manufacturer != "" {
			b.WriteString("\n**SMBIOS**\n")
			if s.BIOSVersion != "" {
				fmt.Fprintf(&b, "- bios version: `%s`\n", s.BIOSVersion)
			}
			if s.SerialNumber != "" {
				fmt.Fprintf(&b, "- serial: `%s`\n", s.SerialNumber)
			}
			if s.Manufacturer != "" {
				fmt.Fprintf(&b, "- manufacturer: `%s`\n", s.Manufacturer)
			}
		}
	}

	// — Security —
	if info.Security.TPMEnabled != "" {
		b.WriteString("\n### Security\n\n")
		fmt.Fprintf(&b, "- **tpm**: %s", info.Security.TPMEnabled)
		if info.Security.TPMType != "" {
			fmt.Fprintf(&b, " (%s)", info.Security.TPMType)
		}
		b.WriteString("\n")
		if info.Security.Encrypted != "" {
			fmt.Fprintf(&b, "- **encrypted**: %s\n", info.Security.Encrypted)
		}
		if info.Security.Protected != "" {
			fmt.Fprintf(&b, "- **protected**: %s\n", info.Security.Protected)
		}
		if info.Security.Locked != "" {
			fmt.Fprintf(&b, "- **config locked**: %s\n", info.Security.Locked)
		}
		if info.Security.Archived != "" && info.Security.Archived != "no" {
			fmt.Fprintf(&b, "- **archived**: %s\n", info.Security.Archived)
		}
		if info.Security.Packed != "" && info.Security.Packed != "no" {
			fmt.Fprintf(&b, "- **packed**: %s\n", info.Security.Packed)
		}
	}

	// — Guest Tools —
	if info.GuestTools.State != "" {
		b.WriteString("\n### Guest Tools\n\n")
		fmt.Fprintf(&b, "- **state**: %s", info.GuestTools.State)
		if info.GuestTools.Version != "" {
			fmt.Fprintf(&b, " (%s)", info.GuestTools.Version)
		}
		b.WriteString("\n")
	}

	// — CPU / Memory —
	b.WriteString("\n### Hardware\n\n")
	if cpu := info.CPUDetails(); cpu != nil {
		fmt.Fprintf(&b, "- **cpu**: %d vCPU (arch: %s, accel: %s, mode: %s, VT-x: %v)\n",
			cpu.CPUs, or(cpu.Type, "—"), or(cpu.Accl, "—"), or(cpu.Mode, "—"), cpu.VTx)
	} else {
		fmt.Fprintf(&b, "- **cpu**: %d vCPU\n", info.CPUs())
	}
	if mb := info.MemoryMB(); mb > 0 {
		fmt.Fprintf(&b, "- **memory**: %d MB (%.1f GB)\n", mb, float64(mb)/1024)
	}

	// — USB —
	u := info.USBBluetooth
	if u.USB30 != "" {
		fmt.Fprintf(&b, "- **usb 3.0**: %s\n", u.USB30)
	}
	if u.ShareCameras != "" {
		fmt.Fprintf(&b, "- **share cameras**: %s\n", u.ShareCameras)
	}
	if u.ShareBluetooth != "" {
		fmt.Fprintf(&b, "- **share bluetooth**: %s\n", u.ShareBluetooth)
	}
	if u.ShareGamepads != "" {
		fmt.Fprintf(&b, "- **share gamepads**: %s\n", u.ShareGamepads)
	}

	// — Sound —
	if snd := info.Sound(); snd != nil && snd.Enabled {
		fmt.Fprintf(&b, "- **sound**: output=%s, mixer=%s\n", or(snd.Output, "—"), or(snd.Mixer, "—"))
	}

	// — Video —
	if vid := info.Video(); vid != nil {
		b.WriteString("\n### Video\n\n")
		b.WriteString("| adapter | 3d-accel | high-res | auto-mem |\n|---|---|---|---|\n")
		fmt.Fprintf(&b, "| %s | %s | %s | %s |\n",
			or(vid.AdapterType, "—"),
			or(vid.ThreeDAccel, "—"),
			or(vid.HighResolution, "—"),
			or(vid.AutoMem, "—"),
		)
	}

	// — Disks —
	if disks := info.Disks(); len(disks) > 0 {
		b.WriteString("\n### Disks\n\n| port | type | size | compact | enabled | image |\n|---|---|---|---|---|---|\n")
		for _, d := range disks {
			fmt.Fprintf(&b, "| %s | %s | %s | %s | %v | `%s` |\n",
				or(d.Port, "—"), or(d.Type, "—"), or(d.Size, "—"),
				or(d.OnlineCompact, "—"), d.Enabled, shortPath(d.Image))
		}
	}

	// — CD-ROMs —
	if cdroms := info.CDROMs(); len(cdroms) > 0 {
		b.WriteString("\n### CD-ROMs\n\n| port | state | enabled | image |\n|---|---|---|---|\n")
		for _, c := range cdroms {
			fmt.Fprintf(&b, "| %s | %s | %v | `%s` |\n",
				or(c.Port, "—"), or(c.State, "—"), c.Enabled, shortPath(c.Image))
		}
	}

	// — Serial Ports —
	if serials := info.Serials(); len(serials) > 0 {
		b.WriteString("\n### Serial Ports\n\n| socket | mode | enabled |\n|---|---|---|\n")
		for _, s := range serials {
			fmt.Fprintf(&b, "| `%s` | %s | %v |\n", s.Socket, or(s.Mode, "—"), s.Enabled)
		}
	}

	// — Network —
	if nets := info.Nets(); len(nets) > 0 {
		conditioned := ""
		if info.Network.Conditioned != "" && info.Network.Conditioned != "off" {
			conditioned = fmt.Sprintf(" _(conditioned: %s)_", info.Network.Conditioned)
		}
		fmt.Fprintf(&b, "\n### Network%s\n\n| # | type | card | iface | mac | enabled |\n|---|---|---|---|---|---|\n", conditioned)
		for i, n := range nets {
			fmt.Fprintf(&b, "| net%d | %s | %s | %s | `%s` | %v |\n",
				i, or(n.Type, "—"), or(n.Card, "—"), or(n.Iface, "—"), n.MAC, n.Enabled)
		}
	}

	// — Guest IPs —
	if ips := info.IPv4s(); len(ips) > 0 {
		b.WriteString("\n### Guest IPv4 addresses\n\n")
		for _, ip := range ips {
			fmt.Fprintf(&b, "- `%s`\n", ip)
		}
	}

	// — Time Synchronization —
	ts := info.TimeSyncronization
	if ts.Enabled || ts.Interval > 0 {
		b.WriteString("\n### Time Synchronization\n\n")
		fmt.Fprintf(&b, "- **enabled**: %v\n", ts.Enabled)
		if ts.Interval > 0 {
			fmt.Fprintf(&b, "- **interval**: %ds\n", ts.Interval)
		}
		if ts.SmartMode != "" {
			fmt.Fprintf(&b, "- **smart mode**: %s\n", ts.SmartMode)
		}
	}

	// — Startup & Shutdown —
	if info.StartupShutdown.Autostart != "" {
		b.WriteString("\n### Startup & Shutdown\n\n")
		ss := info.StartupShutdown
		fmt.Fprintf(&b, "- **autostart**: %s\n", ss.Autostart)
		fmt.Fprintf(&b, "- **autostop**: %s\n", ss.Autostop)
		if ss.OnShutdown != "" {
			fmt.Fprintf(&b, "- **on shutdown**: %s\n", ss.OnShutdown)
		}
		if ss.OnWindowClose != "" {
			fmt.Fprintf(&b, "- **on window close**: %s\n", ss.OnWindowClose)
		}
		if ss.PauseIdle != "" {
			fmt.Fprintf(&b, "- **pause idle**: %s\n", ss.PauseIdle)
		}
		if ss.UndoDisks != "" {
			fmt.Fprintf(&b, "- **undo disks**: %s\n", ss.UndoDisks)
		}
	}

	// — Input & Printers —
	mk := info.MouseKeyboard
	pm := info.PrintManagement
	if mk.SmartMouse != "" || pm.SyncPrinters != "" {
		b.WriteString("\n### Input & Printers\n\n")
		if mk.SmartMouse != "" {
			fmt.Fprintf(&b, "- **smart mouse**: %s\n", mk.SmartMouse)
		}
		if mk.SmoothScrolling != "" {
			fmt.Fprintf(&b, "- **smooth scrolling**: %s\n", mk.SmoothScrolling)
		}
		if mk.KeyboardMode != "" {
			fmt.Fprintf(&b, "- **keyboard mode**: %s\n", mk.KeyboardMode)
		}
		if pm.SyncPrinters != "" {
			fmt.Fprintf(&b, "- **sync printers**: %s\n", pm.SyncPrinters)
		}
	}

	// — Integration & Sharing —
	sp := info.SharedProfile
	sa := info.SharedApps
	sm := info.SmartMount
	mc := info.MiscSharing
	if mc.SharedClipboardMode != "" || sp.Enabled || sa.Enabled {
		b.WriteString("\n### Integration & Sharing\n\n")
		if mc.SharedClipboardMode != "" {
			fmt.Fprintf(&b, "- **shared clipboard**: %s\n", mc.SharedClipboardMode)
		}
		if mc.SharedCloud != "" {
			fmt.Fprintf(&b, "- **shared cloud**: %s\n", mc.SharedCloud)
		}
		if sp.Enabled {
			fmt.Fprintf(&b, "- **shared profile**: enabled (desktop=%s, docs=%s, downloads=%s)\n",
				or(sp.UseDesktop, "—"), or(sp.UseDocuments, "—"), or(sp.UseDownloads, "—"))
		}
		if sa.Enabled {
			fmt.Fprintf(&b, "- **shared apps**: enabled (host->guest=%s, guest->host=%s)\n",
				or(sa.HostToGuest, "—"), or(sa.GuestToHost, "—"))
		}
		if sm.Enabled {
			fmt.Fprintf(&b, "- **smart mount**: enabled (removable=%s, cd/dvd=%s, net=%s)\n",
				or(sm.RemovableDrives, "—"), or(sm.CDDVDDrives, "—"), or(sm.NetworkShares, "—"))
		}
	}

	// — Advanced —
	adv := info.Advanced
	if adv.HostnameSync != "" || adv.SSHKeysSync != "" {
		b.WriteString("\n### Advanced\n\n")
		if adv.HostnameSync != "" {
			fmt.Fprintf(&b, "- **hostname sync**: %s\n", adv.HostnameSync)
		}
		if adv.SSHKeysSync != "" {
			fmt.Fprintf(&b, "- **ssh keys sync**: %s\n", adv.SSHKeysSync)
		}
		if adv.DeveloperTools != "" {
			fmt.Fprintf(&b, "- **developer tools**: %s\n", adv.DeveloperTools)
		}
		if adv.RosettaLinux != "" {
			fmt.Fprintf(&b, "- **rosetta linux**: %s\n", adv.RosettaLinux)
		}
		if adv.ShareLocation != "" {
			fmt.Fprintf(&b, "- **share location**: %s\n", adv.ShareLocation)
		}
	}

	// — Smart Guard —
	if info.SmartGuard.Enabled {
		b.WriteString("\n### Smart Guard\n\n- **enabled**: true\n")
	}

	// — Host Shared Folders —
	if sfs := info.SharedFolders(); len(sfs) > 0 {
		b.WriteString("\n### Host Shared Folders\n\n| name | path | mode | enabled |\n|---|---|---|---|\n")
		for _, sf := range sfs {
			fmt.Fprintf(&b, "| %s | `%s` | %s | %v |\n", sf.Name, sf.Path, or(sf.Mode, "—"), sf.Enabled)
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
