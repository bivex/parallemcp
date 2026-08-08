package parallels

import (
	"context"
	"errors"
	"fmt"
	"net"
	"os/exec"
	"strings"
	"time"
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

// KernelDebugParams options for automated kernel debugging setup.
type KernelDebugParams struct {
	Mode        string // "net" (KDNET for Windows), "serial" (COM socket), "gdb" (Parallels GDB)
	Port        int    // KDNET or GDB port (default: 50000)
	Key         string // KDNET key (default: 1.2.3.4)
	HostIP      string // Host IP for KDNET (default: 10.211.55.2)
	SocketPath  string // Unix socket for serial COM port (default: /tmp/<vm>_kd.sock)
	AutoBcdedit bool   // Automatically execute bcdedit inside Windows VM if running
}

// KernelDebugResult contains the setup results and generated commands.
type KernelDebugResult struct {
	Mode         string
	SocketPath   string
	KDNETKey     string
	KDNETPort    int
	HostIP       string
	BcdeditDone  bool
	BcdeditError string
	ConnectCmd   string
}

// ConfigureKernelDebug automates kernel debugging setup for vmID.
func (c *Client) ConfigureKernelDebug(ctx context.Context, id string, p KernelDebugParams) (*KernelDebugResult, error) {
	mode := strings.ToLower(strings.TrimSpace(p.Mode))
	if mode == "" {
		mode = "net"
	}

	res := &KernelDebugResult{
		Mode:       mode,
		KDNETPort:  p.Port,
		KDNETKey:   p.Key,
		HostIP:     p.HostIP,
		SocketPath: p.SocketPath,
	}

	if res.KDNETPort <= 0 {
		res.KDNETPort = 50000
	}
	if res.KDNETKey == "" {
		res.KDNETKey = "1.2.3.4"
	}
	if res.HostIP == "" {
		res.HostIP = "10.211.55.2" // Standard Parallels NAT Host IP
	}
	// Fetch VM info to inspect current devices, home path, and OS
	info, _ := c.Info(ctx, id)
	osType := ""
	if info != nil {
		osType = strings.ToLower(info.OS)
	}

	if res.SocketPath == "" {
		if info != nil && info.HomePath != "" {
			// Place socket inside .pvm bundle directory to ensure user file ownership
			pvmDir := info.HomePath
			if strings.HasSuffix(pvmDir, "/config.pvs") {
				pvmDir = strings.TrimSuffix(pvmDir, "/config.pvs")
			}
			res.SocketPath = pvmDir + "/kd.sock"
		} else {
			cleanVM := strings.NewReplacer(" ", "_", "/", "_", "\\", "_", ":", "_").Replace(id)
			res.SocketPath = fmt.Sprintf("/tmp/%s_kd.sock", cleanVM)
		}
	}


	switch mode {
	case "serial":
		// Ensure serial port device with socket exists
		_ = c.ok(ctx, Prlctl, "set", id, "--device-add", "serial", "--socket", res.SocketPath, "--socket-mode", "server")
		if p.AutoBcdedit && strings.Contains(osType, "win") {
			cmd := "bcdedit /debug on && bcdedit /dbgsettings serial debugport:1 baudrate:115200"
			r, err := c.ExecOS(ctx, id, cmd, info.OS)
			if err == nil && r.ExitCode == 0 {
				res.BcdeditDone = true
			} else if err != nil {
				res.BcdeditError = err.Error()
			} else {
				res.BcdeditError = strings.TrimSpace(r.Stderr)
			}
		}
		res.ConnectCmd = fmt.Sprintf("gdb vmlinux -ex \"target remote %s\"", res.SocketPath)

	case "gdb":
		// Configure SystemFlags for GDB stub
		_ = c.ConfigureDebugger(ctx, id, VMDebugConfig{
			Enabled:   true,
			Protocol:  DebugProtocolGDB,
			LocalAddr: "127.0.0.1",
		})
		// Also add serial socket for KGDB fallback
		_ = c.ok(ctx, Prlctl, "set", id, "--device-add", "serial", "--socket", res.SocketPath, "--socket-mode", "server")
		res.ConnectCmd = fmt.Sprintf("gdb vmlinux -ex \"target remote 127.0.0.1:%d\"", res.KDNETPort)

	case "net", "kdnet":
		// Windows KDNET mode
		if p.AutoBcdedit {
			cmd := fmt.Sprintf("bcdedit /debug on && bcdedit /dbgsettings net hostip:%s port:%d key:%s",
				res.HostIP, res.KDNETPort, res.KDNETKey)
			r, err := c.ExecOS(ctx, id, cmd, "win-11")
			if err == nil && r.ExitCode == 0 {
				res.BcdeditDone = true
			} else if err != nil {
				res.BcdeditError = err.Error()
			} else {
				res.BcdeditError = strings.TrimSpace(r.Stderr)
			}
		}
		res.ConnectCmd = fmt.Sprintf("windbg -k net:port=%d,key=%s,target=%s", res.KDNETPort, res.KDNETKey, res.HostIP)

	default:
		return nil, fmt.Errorf("unsupported mode: %s (must be 'net', 'serial', or 'gdb')", mode)
	}

	return res, nil
}

// DebugGDBExec runs GDB batch commands targeting a remote socket or TCP endpoint.
func (c *Client) DebugGDBExec(ctx context.Context, target string, arch string, gdbCmds []string) (string, error) {
	if strings.TrimSpace(target) == "" {
		return "", errors.New("target is required")
	}
	gdbBin := "gdb"
	if _, err := exec.LookPath("/opt/homebrew/bin/gdb"); err == nil {
		gdbBin = "/opt/homebrew/bin/gdb"
	}


	if arch == "" {
		arch = "aarch64"
	}

	args := []string{"-batch", "-ex", "set architecture " + arch}
	if target != "" {
		args = append(args, "-ex", "target remote "+target)
	}
	for _, cmd := range gdbCmds {
		args = append(args, "-ex", cmd)
	}

	r, err := c.exec(ctx, gdbBin, args...)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(r.Stdout + "\n" + r.Stderr), nil
}

// DebugSerialReadWrite reads/writes data over a UNIX domain serial socket without Python.
func (c *Client) DebugSerialReadWrite(ctx context.Context, socketPath string, sendStr string, timeout time.Duration) (string, error) {
	if strings.TrimSpace(socketPath) == "" {
		return "", errors.New("socket_path is required")
	}
	if timeout <= 0 {
		timeout = 2 * time.Second
	}
	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return "", fmt.Errorf("dial unix socket %s: %w", socketPath, err)
	}
	defer conn.Close()

	_ = conn.SetDeadline(time.Now().Add(timeout))

	if sendStr != "" {
		if _, err := conn.Write([]byte(sendStr)); err != nil {
			return "", fmt.Errorf("write to socket: %w", err)
		}
	}

	buf := make([]byte, 4096)
	n, _ := conn.Read(buf)
	return string(buf[:n]), nil
}



