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
