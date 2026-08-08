package parallels

import (
	"context"
	"testing"
)

func TestExtraOps(t *testing.T) {
	stub := newStub(map[string]string{
		"prlctl capture vm -f /tmp/snap.png":                            "",
		"prlctl set vm --online-compact on":                             "",
		"prlctl set vm --device-set cdrom0 --image boot.iso --connect": "",
		"prlctl set vm --device-set cdrom0 --disconnect":                "",
		"prlctl set vm --device-del cdrom0":                             "",
		"prlctl snapshot-delete vm -i snap1":                            "",
		"prlctl set vm --device-add hdd --size 64G":                     "",
		"prlctl set vm --device-set hdd0 --size 128G":                   "",
		"prlctl installtools vm":                                        "",
		"prlctl set vm --device-add net --type shared":                  "",
		"prlctl set vm --device-del net1":                               "",
		"prlctl register /tmp/vm.pvm":                                   "",
		"prlctl unregister vm":                                          "",
		"prlctl set vm --system-flags vm.debug=1&vm.debug.protocol=0&vm.debug.local_addr=127.0.0.1": "",
	})

	c := &Client{Run: stub}
	ctx := context.Background()

	if err := c.Capture(ctx, "vm", "/tmp/snap.png"); err != nil {
		t.Errorf("Capture: %v", err)
	}
	if err := c.Compact(ctx, "vm"); err != nil {
		t.Errorf("Compact: %v", err)
	}
	if err := c.CDROM(ctx, "vm", CDROMParams{Device: "cdrom0", Action: "attach", Image: "boot.iso"}); err != nil {
		t.Errorf("CDROM attach: %v", err)
	}
	if err := c.CDROM(ctx, "vm", CDROMParams{Device: "cdrom0", Action: "disconnect"}); err != nil {
		t.Errorf("CDROM disconnect: %v", err)
	}
	if err := c.CDROM(ctx, "vm", CDROMParams{Device: "cdrom0", Action: "detach"}); err != nil {
		t.Errorf("CDROM detach: %v", err)
	}
	if err := c.SnapshotDelete(ctx, "vm", "snap1"); err != nil {
		t.Errorf("SnapshotDelete: %v", err)
	}
	if err := c.DiskManage(ctx, "vm", DiskManageParams{Action: "add", Size: "64G"}); err != nil {
		t.Errorf("DiskManage add: %v", err)
	}
	if err := c.DiskManage(ctx, "vm", DiskManageParams{Action: "resize", Device: "hdd0", Size: "128G"}); err != nil {
		t.Errorf("DiskManage resize: %v", err)
	}
	if err := c.InstallTools(ctx, "vm"); err != nil {
		t.Errorf("InstallTools: %v", err)
	}
	if err := c.DeviceManage(ctx, "vm", DeviceManageParams{Action: "add", DeviceType: "net"}); err != nil {
		t.Errorf("DeviceManage add net: %v", err)
	}
	if err := c.DeviceManage(ctx, "vm", DeviceManageParams{Action: "remove", DeviceName: "net1"}); err != nil {
		t.Errorf("DeviceManage remove net1: %v", err)
	}
	if err := c.RegisterBundle(ctx, "/tmp/vm.pvm"); err != nil {
		t.Errorf("RegisterBundle: %v", err)
	}
	if err := c.UnregisterBundle(ctx, "vm"); err != nil {
		t.Errorf("UnregisterBundle: %v", err)
	}
	if err := c.ConfigureDebugger(ctx, "vm", VMDebugConfig{Enabled: true, Protocol: DebugProtocolGDB, LocalAddr: "127.0.0.1"}); err != nil {
		t.Errorf("ConfigureDebugger: %v", err)
	}
}

func TestConfigureKernelDebug(t *testing.T) {
	stub := newStub(map[string]string{
		"prlctl set vm --device-add serial --socket /tmp/vm_kd.sock --socket-mode server": "",
		"prlctl set vm --system-flags vm.debug=1&vm.debug.protocol=0&vm.debug.local_addr=127.0.0.1": "",
	})
	c := &Client{Run: stub}
	ctx := context.Background()

	// Mode: serial
	resSerial, err := c.ConfigureKernelDebug(ctx, "vm", KernelDebugParams{Mode: "serial", SocketPath: "/tmp/vm_kd.sock"})
	if err != nil {
		t.Fatalf("ConfigureKernelDebug serial: %v", err)
	}
	if resSerial.SocketPath != "/tmp/vm_kd.sock" {
		t.Errorf("expected socket path /tmp/vm_kd.sock, got %s", resSerial.SocketPath)
	}

	// Mode: gdb
	resGDB, err := c.ConfigureKernelDebug(ctx, "vm", KernelDebugParams{Mode: "gdb", SocketPath: "/tmp/vm_kd.sock", Port: 50000})
	if err != nil {
		t.Fatalf("ConfigureKernelDebug gdb: %v", err)
	}
	if resGDB.KDNETPort != 50000 {
		t.Errorf("expected port 50000, got %d", resGDB.KDNETPort)
	}

	// Edge case: invalid mode
	_, err = c.ConfigureKernelDebug(ctx, "vm", KernelDebugParams{Mode: "invalid_mode"})
	if err == nil {
		t.Errorf("expected error for invalid mode, got nil")
	}

	// Edge case: DebugGDBExec empty target
	_, err = c.DebugGDBExec(ctx, "", "aarch64", nil)
	if err == nil {
		t.Errorf("expected error for empty target in DebugGDBExec, got nil")
	}

	// Edge case: DebugSerialReadWrite empty socket path
	_, err = c.DebugSerialReadWrite(ctx, "", "", 0)
	if err == nil {
		t.Errorf("expected error for empty socket path in DebugSerialReadWrite, got nil")
	}
}


func TestGuestDir(t *testing.T) {
	winPath := `C:\Temp\Folder\file.txt`
	dirWin := guestDir(winPath, true)
	if dirWin != `C:\Temp\Folder` {
		t.Errorf("guestDir Windows = %q, want %q", dirWin, `C:\Temp\Folder`)
	}

	unixPath := `/tmp/folder/file.txt`
	dirUnix := guestDir(unixPath, false)
	if dirUnix != `/tmp/folder` {
		t.Errorf("guestDir Unix = %q, want %q", dirUnix, `/tmp/folder`)
	}
}
