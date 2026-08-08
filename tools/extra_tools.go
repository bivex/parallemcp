package tools

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func (t *Tools) registerExtraTools(s *mcp.Server) {
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_screenshot",
		Description: "Capture a PNG screenshot of a VM's display.",
	}, t.vmScreenshot)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_compact",
		Description: "Compact a VM's virtual disk (.hdd) to reclaim host disk space.",
	}, t.vmCompact)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_cdrom",
		Description: "Attach, connect, disconnect, or detach ISO images in a VM's CD/DVD drive.",
	}, t.vmCDROM)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_snapshot_delete",
		Description: "Delete a snapshot of a VM to free disk space.",
	}, t.vmSnapshotDelete)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_disk_manage",
		Description: "Add a new virtual hard disk or resize an existing disk.",
	}, t.vmDiskManage)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_install_tools",
		Description: "Mount Parallels Guest Tools ISO into the guest VM for installation.",
	}, t.vmInstallTools)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_device",
		Description: "Add or remove hardware devices (network adapters, serial ports, sound cards).",
	}, t.vmDevice)
	mcp.AddTool(s, &mcp.Tool{
		Name:        "vm_bundle",
		Description: "Register an existing .pvm bundle path or unregister a VM without deleting files.",
	}, t.vmBundle)
	mcp.AddTool(s, &mcp.Tool{
		Name: "vm_configure_debugger",
		Description: "Enable or disable the Parallels built-in guest debugger interface (GDB or KDBG/WinDbg) " +
			"by writing vm.debug SystemFlags to config.pvs. The VM must be stopped first. " +
			"After enabling, start the VM and call vm_guest_debugger to connect.",
	}, t.vmConfigureDebugger)
	mcp.AddTool(s, &mcp.Tool{
		Name: "vm_guest_debugger",
		Description: "Attach the Parallels guest debugger to a running VM and return the host:port " +
			"that GDB / WinDbg should connect to. Requires the debugger interface to be pre-enabled " +
			"via vm_configure_debugger before the VM was started.",
	}, t.vmGuestDebugger)
	mcp.AddTool(s, &mcp.Tool{
		Name: "vm_configure_kernel_debug",
		Description: "Automate kernel debugging setup for a VM (Windows KDNET, Serial COM Socket, or GDB). " +
			"Automatically configures serial COM devices, Parallels GDB flags, and (if VM is running Windows) " +
			"runs bcdedit inside the guest VM to enable kernel debugging.",
	}, t.vmConfigureKernelDebug)
}


type vmScreenshotInput struct {
	VM       string `json:"vm" jsonschema:"VM name or UUID"`
	FilePath string `json:"file_path,omitempty" jsonschema:"destination PNG file path on host"`
}

func (t *Tools) vmScreenshot(ctx context.Context, req *mcp.CallToolRequest, in vmScreenshotInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	path := in.FilePath
	if path == "" {
		path = fmt.Sprintf("/tmp/vm_screenshot_%s_%d.png", sanitize(in.VM), time.Now().Unix())
	}
	if err := t.cli.Capture(ctx, in.VM, path); err != nil {
		return fail("capture screenshot", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("📸 Captured screenshot of **%s** ➔ `%s`.", in.VM, path)), noOut{}, nil
}

type vmCompactInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmCompact(ctx context.Context, req *mcp.CallToolRequest, in vmCompactInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.Compact(ctx, in.VM); err != nil {
		return fail("compact disk", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("🗜️ Compacted virtual disk for **%s**.", in.VM)), noOut{}, nil
}

type vmCDROMInput struct {
	VM        string `json:"vm" jsonschema:"VM name or UUID"`
	Action    string `json:"action" jsonschema:"action: 'attach', 'connect', 'disconnect', or 'detach'"`
	ImagePath string `json:"image_path,omitempty" jsonschema:"ISO image file path for attach/connect"`
	Device    string `json:"device,omitempty" jsonschema:"CD/DVD device name (default 'cdrom0')"`
}

func (t *Tools) vmCDROM(ctx context.Context, req *mcp.CallToolRequest, in vmCDROMInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.Action == "" {
		return errResult("`vm` and `action` are required"), noOut{}, nil
	}
	dev := in.Device
	if dev == "" {
		dev = "cdrom0"
	}
	err := t.cli.CDROM(ctx, in.VM, parallels.CDROMParams{
		Device: dev,
		Action: in.Action,
		Image:  in.ImagePath,
	})
	if err != nil {
		return fail("cdrom operation", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("💿 CD/DVD drive **%s** on **%s**: action '%s' applied.", dev, in.VM, in.Action)), noOut{}, nil
}

type vmSnapshotDeleteInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
	ID string `json:"id" jsonschema:"snapshot ID or name to delete"`
}

func (t *Tools) vmSnapshotDelete(ctx context.Context, req *mcp.CallToolRequest, in vmSnapshotDeleteInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.ID == "" {
		return errResult("`vm` and `id` are required"), noOut{}, nil
	}
	if err := t.cli.SnapshotDelete(ctx, in.VM, in.ID); err != nil {
		return fail("delete snapshot", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("🗑️ Deleted snapshot **%s** from **%s**.", in.ID, in.VM)), noOut{}, nil
}

type vmDiskManageInput struct {
	VM     string `json:"vm" jsonschema:"VM name or UUID"`
	Action string `json:"action" jsonschema:"action: 'add' or 'resize'"`
	Device string `json:"device,omitempty" jsonschema:"disk device name for resize (default 'hdd0')"`
	Size   string `json:"size" jsonschema:"disk size (e.g. '64G' or '65536Mb')"`
}

func (t *Tools) vmDiskManage(ctx context.Context, req *mcp.CallToolRequest, in vmDiskManageInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.Action == "" || in.Size == "" {
		return errResult("`vm`, `action`, and `size` are required"), noOut{}, nil
	}
	dev := in.Device
	if dev == "" {
		dev = "hdd0"
	}
	err := t.cli.DiskManage(ctx, in.VM, parallels.DiskManageParams{
		Action: in.Action,
		Device: dev,
		Size:   in.Size,
	})
	if err != nil {
		return fail("disk operation", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("💾 Disk action '%s' (%s) applied to **%s**.", in.Action, in.Size, in.VM)), noOut{}, nil
}

type vmInstallToolsInput struct {
	VM string `json:"vm" jsonschema:"VM name or UUID"`
}

func (t *Tools) vmInstallTools(ctx context.Context, req *mcp.CallToolRequest, in vmInstallToolsInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	if err := t.cli.InstallTools(ctx, in.VM); err != nil {
		return fail("install tools", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("🛠️ Mounted Parallels Guest Tools installer for **%s**.", in.VM)), noOut{}, nil
}

type vmDeviceInput struct {
	VM         string `json:"vm" jsonschema:"VM name or UUID"`
	Action     string `json:"action" jsonschema:"action: 'add' or 'remove'"`
	DeviceType string `json:"device_type,omitempty" jsonschema:"device type for add ('net', 'serial', 'sound')"`
	DeviceName string `json:"device_name,omitempty" jsonschema:"device name for remove (e.g. 'net1', 'serial0')"`
}

func (t *Tools) vmDevice(ctx context.Context, req *mcp.CallToolRequest, in vmDeviceInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" || in.Action == "" {
		return errResult("`vm` and `action` are required"), noOut{}, nil
	}
	err := t.cli.DeviceManage(ctx, in.VM, parallels.DeviceManageParams{
		Action:     in.Action,
		DeviceType: in.DeviceType,
		DeviceName: in.DeviceName,
	})
	if err != nil {
		return fail("device operation", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("🔌 Device action '%s' applied to **%s**.", in.Action, in.VM)), noOut{}, nil
}

type vmBundleInput struct {
	Action string `json:"action" jsonschema:"action: 'register' or 'unregister'"`
	Path   string `json:"path,omitempty" jsonschema:"path to .pvm bundle for 'register'"`
	VM     string `json:"vm,omitempty" jsonschema:"VM name or UUID for 'unregister'"`
}

func (t *Tools) vmBundle(ctx context.Context, req *mcp.CallToolRequest, in vmBundleInput) (*mcp.CallToolResult, noOut, error) {
	switch strings.ToLower(in.Action) {
	case "register":
		if in.Path == "" {
			return errResult("`path` is required for register"), noOut{}, nil
		}
		if err := t.cli.RegisterBundle(ctx, in.Path); err != nil {
			return fail("register bundle", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("🗄️ Registered VM bundle from `%s`.", in.Path)), noOut{}, nil
	case "unregister":
		if in.VM == "" {
			return errResult("`vm` is required for unregister"), noOut{}, nil
		}
		if err := t.cli.UnregisterBundle(ctx, in.VM); err != nil {
			return fail("unregister bundle", err), noOut{}, nil
		}
		return textResult(fmt.Sprintf("🗄️ Unregistered VM **%s** (files preserved).", in.VM)), noOut{}, nil
	default:
		return errResult("`action` must be 'register' or 'unregister'"), noOut{}, nil
	}
}

func sanitize(s string) string {
	r := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_")
	return r.Replace(s)
}
// ── vm_configure_debugger ────────────────────────────────────────────────────

type vmConfigureDebuggerInput struct {
	VM        string `json:"vm" jsonschema:"VM name or UUID (must be stopped)"`
	Enable    bool   `json:"enable" jsonschema:"true to enable the debugger interface, false to disable"`
	Protocol  string `json:"protocol,omitempty" jsonschema:"wire protocol: 'gdb' (default) or 'kdbg' (WinDbg/KDBG)"`
	LocalAddr string `json:"local_addr,omitempty" jsonschema:"host-side IP to bind the debug socket (default: 127.0.0.1)"`
}

func (t *Tools) vmConfigureDebugger(ctx context.Context, req *mcp.CallToolRequest, in vmConfigureDebuggerInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	cfg := parallels.VMDebugConfig{
		Enabled:   in.Enable,
		LocalAddr: in.LocalAddr,
	}
	switch strings.ToLower(in.Protocol) {
	case "", "gdb":
		cfg.Protocol = parallels.DebugProtocolGDB
	case "kdbg", "windbg":
		cfg.Protocol = parallels.DebugProtocolKDBG
	default:
		return errResult("protocol must be 'gdb' or 'kdbg'"), noOut{}, nil
	}
	if err := t.cli.ConfigureDebugger(ctx, in.VM, cfg); err != nil {
		return fail("configure debugger", err), noOut{}, nil
	}
	if !in.Enable {
		return textResult(fmt.Sprintf("✅ Guest debugger **disabled** for **%s**.", in.VM)), noOut{}, nil
	}
	addr := in.LocalAddr
	if addr == "" {
		addr = "127.0.0.1"
	}
	proto := "GDB"
	if cfg.Protocol == parallels.DebugProtocolKDBG {
		proto = "KDBG / WinDbg"
	}
	msg := fmt.Sprintf(
		"✅ Guest debugger **enabled** for **%s**.\n\n"+
			"- **Protocol:** %s\n"+
			"- **Host bind addr:** `%s`\n\n"+
			"Start the VM, then call `vm_guest_debugger` to obtain the connect address and port.\n\n"+
			"**GDB connect example:**\n"+
			"```\ngdb vmlinux\n(gdb) target remote %s:<port>\n```\n\n"+
			"**WinDbg connect example:**\n"+
			"```\nwindbg -k net:port=<port>,target=%s\n```",
		in.VM, proto, addr, addr, addr,
	)
	return textResult(msg), noOut{}, nil
}

// ── vm_guest_debugger ────────────────────────────────────────────────────────

type vmGuestDebuggerInput struct {
	VM   string `json:"vm" jsonschema:"VM name or UUID (must be running with debugger enabled)"`
	Port int    `json:"port,omitempty" jsonschema:"TCP port to use (0 = let Parallels choose)"`
}

func (t *Tools) vmGuestDebugger(ctx context.Context, req *mcp.CallToolRequest, in vmGuestDebuggerInput) (*mcp.CallToolResult, noOut, error) {

	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	out, err := t.cli.GuestDebugger(ctx, in.VM, parallels.GuestDebuggerParams{Port: in.Port})
	if err != nil {
		return fail("guest-debugger", err), noOut{}, nil
	}
	return textResult(fmt.Sprintf("## Guest Debugger: %s\n\n```\n%s\n```", in.VM, strings.TrimSpace(out))), noOut{}, nil
}

// ── vm_configure_kernel_debug ────────────────────────────────────────────────

type vmConfigureKernelDebugInput struct {
	VM          string `json:"vm" jsonschema:"VM name or UUID"`
	Mode        string `json:"mode,omitempty" jsonschema:"debugging mode: 'net' (KDNET Windows), 'serial' (COM socket), 'gdb' (Parallels GDB stub)"`
	Port        int    `json:"port,omitempty" jsonschema:"TCP port for KDNET or GDB (default: 50000)"`
	Key         string `json:"key,omitempty" jsonschema:"KDNET encryption key for Windows (default: 1.2.3.4)"`
	HostIP      string `json:"host_ip,omitempty" jsonschema:"Host IP address for KDNET (default: 10.211.55.2)"`
	SocketPath  string `json:"socket_path,omitempty" jsonschema:"Unix socket path for serial port (default: /tmp/<vm>_kd.sock)"`
	AutoBcdedit bool   `json:"auto_bcdedit,omitempty" jsonschema:"automatically run bcdedit inside Windows guest VM"`
}

func (t *Tools) vmConfigureKernelDebug(ctx context.Context, req *mcp.CallToolRequest, in vmConfigureKernelDebugInput) (*mcp.CallToolResult, noOut, error) {
	if in.VM == "" {
		return errResult("`vm` is required"), noOut{}, nil
	}
	p := parallels.KernelDebugParams{
		Mode:        in.Mode,
		Port:        in.Port,
		Key:         in.Key,
		HostIP:      in.HostIP,
		SocketPath:  in.SocketPath,
		AutoBcdedit: in.AutoBcdedit,
	}
	res, err := t.cli.ConfigureKernelDebug(ctx, in.VM, p)
	if err != nil {
		return fail("configure kernel debug", err), noOut{}, nil
	}

	var b strings.Builder
	fmt.Fprintf(&b, "## Kernel Debugging Configured: %s\n\n", in.VM)
	fmt.Fprintf(&b, "- **Mode:** `%s`\n", res.Mode)

	switch res.Mode {
	case "net", "kdnet":
		fmt.Fprintf(&b, "- **KDNET Port:** `%d`\n", res.KDNETPort)
		fmt.Fprintf(&b, "- **KDNET Key:** `%s`\n", res.KDNETKey)
		fmt.Fprintf(&b, "- **Host IP:** `%s`\n", res.HostIP)
		if in.AutoBcdedit {
			if res.BcdeditDone {
				b.WriteString("- **Guest `bcdedit` status:** ✅ Applied successfully inside Windows guest.\n")
			} else if res.BcdeditError != "" {
				fmt.Fprintf(&b, "- **Guest `bcdedit` status:** ⚠️ Could not run automatically (`%s`). Run manually inside VM.\n", res.BcdeditError)
			}
		} else {
			b.WriteString("\n**Run inside Windows guest (`cmd.exe` as Administrator):**\n```cmd\nbcdedit /debug on\nbcdedit /dbgsettings net hostip:" + res.HostIP + " port:" + fmt.Sprintf("%d", res.KDNETPort) + " key:" + res.KDNETKey + "\n```\n")
		}
		fmt.Fprintf(&b, "\n**WinDbg connect command on Mac host:**\n```\n%s\n```\n", res.ConnectCmd)

	case "serial":
		fmt.Fprintf(&b, "- **COM Socket Path:** `%s`\n", res.SocketPath)
		if in.AutoBcdedit {
			if res.BcdeditDone {
				b.WriteString("- **Guest `bcdedit` status:** ✅ Applied successfully inside Windows guest.\n")
			} else if res.BcdeditError != "" {
				fmt.Fprintf(&b, "- **Guest `bcdedit` status:** ⚠️ Could not run automatically (`%s`).\n", res.BcdeditError)
			}
		}
		b.WriteString("\n**Linux Kernel boot parameter (`cmdline`):**\n```text\nconsole=ttyS0,115200 kgdboc=ttyS0,115200 kgdbwait\n```\n")
		fmt.Fprintf(&b, "\n**GDB connect command on Mac host:**\n```bash\n%s\n```\n", res.ConnectCmd)

	case "gdb":
		fmt.Fprintf(&b, "- **GDB Port:** `%d`\n", res.KDNETPort)
		fmt.Fprintf(&b, "- **Serial Socket:** `%s`\n", res.SocketPath)
		fmt.Fprintf(&b, "\n**GDB connect command on Mac host:**\n```bash\n%s\n```\n", res.ConnectCmd)
	}

	return textResult(b.String()), noOut{}, nil
}

