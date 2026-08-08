package parallels

import (
	"context"
	"errors"
	"fmt"
	"strings"
)


// Capture takes a screenshot of vmID's display and saves it to filePath (PNG format).
func (c *Client) Capture(ctx context.Context, id, filePath string) error {
	if strings.TrimSpace(filePath) == "" {
		return errors.New("file_path is required")
	}
	return c.ok(ctx, Prlctl, "capture", id, "-f", filePath)
}

// Compact enables online disk compacting for vmID's virtual disk files.
func (c *Client) Compact(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "set", id, "--online-compact", "on")
}

// CDROMParams configures VM CD/DVD drive actions.
type CDROMParams struct {
	Device string // e.g. "cdrom0" (default)
	Action string // "attach", "detach", "connect", "disconnect"
	Image  string // ISO path for attach/connect
}

// CDROM attaches, connects, disconnects, or detaches an ISO image.
func (c *Client) CDROM(ctx context.Context, id string, p CDROMParams) error {
	dev := p.Device
	if dev == "" {
		dev = "cdrom0"
	}
	act := strings.ToLower(p.Action)
	switch act {
	case "attach", "connect":
		if strings.TrimSpace(p.Image) == "" {
			return errors.New("image path is required for attach/connect")
		}
		return c.ok(ctx, Prlctl, "set", id, "--device-set", dev, "--image", p.Image, "--connect")
	case "disconnect":
		return c.ok(ctx, Prlctl, "set", id, "--device-set", dev, "--disconnect")
	case "detach", "remove", "delete":
		return c.ok(ctx, Prlctl, "set", id, "--device-del", dev)
	default:
		return errors.New("action must be attach, disconnect, or detach")
	}
}

// DiskManageParams describes adding or resizing a virtual disk.
type DiskManageParams struct {
	Action string // "add" | "resize"
	Device string // e.g. "hdd0" (default for resize)
	Size   string // e.g. "64G", "128G", "65536Mb"
}

// DiskManage adds a new disk or resizes an existing disk.
func (c *Client) DiskManage(ctx context.Context, id string, p DiskManageParams) error {
	if strings.TrimSpace(p.Size) == "" {
		return errors.New("size is required (e.g. '64G' or '65536Mb')")
	}
	switch strings.ToLower(p.Action) {
	case "add", "create":
		return c.ok(ctx, Prlctl, "set", id, "--device-add", "hdd", "--size", p.Size)
	case "resize", "set":
		dev := p.Device
		if dev == "" {
			dev = "hdd0"
		}
		return c.ok(ctx, Prlctl, "set", id, "--device-set", dev, "--size", p.Size)
	default:
		return errors.New("action must be 'add' or 'resize'")
	}
}

// InstallTools mounts Parallels Guest Tools ISO into vmID.
func (c *Client) InstallTools(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "installtools", id)
}

// DeviceManageParams describes adding or removing hardware devices.
type DeviceManageParams struct {
	Action     string // "add" | "remove"
	DeviceType string // "net", "serial", "sound"
	DeviceName string // "net1", "serial0" (for remove)
}

// DeviceManage adds or removes hardware devices.
func (c *Client) DeviceManage(ctx context.Context, id string, p DeviceManageParams) error {
	switch strings.ToLower(p.Action) {
	case "add", "create":
		devType := strings.ToLower(strings.TrimSpace(p.DeviceType))
		if devType == "" {
			return errors.New("device_type is required (e.g. 'net', 'serial', 'sound')")
		}
		args := []string{"set", id, "--device-add", devType}
		if devType == "net" {
			args = append(args, "--type", "shared")
		}
		return c.ok(ctx, Prlctl, args...)
	case "remove", "delete", "del":
		if strings.TrimSpace(p.DeviceName) == "" {
			return errors.New("device_name is required for remove (e.g. 'net1', 'serial0')")
		}
		return c.ok(ctx, Prlctl, "set", id, "--device-del", p.DeviceName)
	default:
		return errors.New("action must be 'add' or 'remove'")
	}
}

// RegisterBundle registers an existing .pvm bundle path.
func (c *Client) RegisterBundle(ctx context.Context, path string) error {
	if strings.TrimSpace(path) == "" {
		return errors.New("path is required")
	}
	return c.ok(ctx, Prlctl, "register", path)
}

// UnregisterBundle unregisters a VM without deleting disk files.
func (c *Client) UnregisterBundle(ctx context.Context, id string) error {
	return c.ok(ctx, Prlctl, "unregister", id)
}

// GuestDebuggerParams holds parameters for the guest-debugger command.
type GuestDebuggerParams struct {
	// Port is the TCP port to expose the GDB/KDBG interface on (e.g. 50000).
	// Parallels defaults to an ephemeral port when 0.
	Port int
}

// GuestDebugger attaches the built-in Parallels guest debugger to a running VM
// and returns the raw prlctl output (which includes the chosen port and address).
// The VM must be running. Use ConfigureDebugger (--system-flags) to enable the
// debugger interface before starting the VM.
func (c *Client) GuestDebugger(ctx context.Context, id string, p GuestDebuggerParams) (string, error) {
	args := []string{"guest-debugger", id}
	if p.Port > 0 {
		args = append(args, "--port", fmt.Sprintf("%d", p.Port))
	}
	r, err := c.exec(ctx, Prlctl, args...)
	if err != nil {
		return "", err
	}
	return r.Stdout, nil
}

// DebugProtocol selects the debugger wire protocol.
type DebugProtocol int

const (
	DebugProtocolGDB  DebugProtocol = 0 // GDB remote serial protocol
	DebugProtocolKDBG DebugProtocol = 1 // Parallels KDBG / WinDbg protocol
)

// VMDebugConfig contains the settings written to SystemFlags in config.pvs.
type VMDebugConfig struct {
	// Enabled enables (true) or disables (false) the guest debugger interface.
	Enabled bool
	// Protocol selects GDB (0) or KDBG/WinDbg (1).
	Protocol DebugProtocol
	// LocalAddr is the host-side IP that Parallels binds the debug socket to.
	// Leave empty to use 127.0.0.1.
	LocalAddr string
}

// ConfigureDebugger writes (or clears) the VM debugger SystemFlags for vmID.
// The VM must be stopped before calling this.
func (c *Client) ConfigureDebugger(ctx context.Context, id string, cfg VMDebugConfig) error {
	var flags string
	if cfg.Enabled {
		addr := cfg.LocalAddr
		if addr == "" {
			addr = "127.0.0.1"
		}
		flags = fmt.Sprintf("vm.debug=1&vm.debug.protocol=%d&vm.debug.local_addr=%s", cfg.Protocol, addr)
	}
	// An empty flags string clears the field.
	return c.ok(ctx, Prlctl, "set", id, "--system-flags", flags)
}

