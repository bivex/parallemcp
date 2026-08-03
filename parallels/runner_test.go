package parallels

import (
	"context"
	"encoding/base64"
	"errors"
	"strings"
	"testing"
)

// stubRunner is a fake Runner that returns canned results keyed by the joined
// command line. It lets the parsing logic be tested without a Parallels install.
type stubRunner struct {
	responses map[string]stubResp
}

type stubResp struct {
	out *CmdResult
	err error
}

func (s *stubRunner) Run(_ context.Context, bin string, args ...string) (*CmdResult, error) {
	key := bin + " " + strings.Join(args, " ")
	r, ok := s.responses[key]
	if !ok {
		return &CmdResult{}, errors.New("stub: unexpected call: " + key)
	}
	if r.out == nil {
		return nil, r.err
	}
	return r.out, r.err
}

func newStub(m map[string]string) *stubRunner {
	s := &stubRunner{responses: map[string]stubResp{}}
	for k, v := range m {
		vv := v // capture
		s.responses[k] = stubResp{out: &CmdResult{Stdout: vv}}
	}
	return s
}

const listSample = `[
  {"uuid":"70c0420f-1717-42e3-85ee-b7f1fd33ac62","status":"suspended","ip_configured":"-","name":"Ubuntu Server ARM64"},
  {"uuid":"62eb5dfb-0002-42b5-8bac-02bf8b02e9df","status":"stopped","ip_configured":"-","name":"Windows 11"},
  {"uuid":"d53f9ec7-88af-4baf-831a-08a68d4dbb9a","status":"running","ip_configured":"-","name":"Windows 11 Pro (Debugger)"}
]`

func TestListAndFind(t *testing.T) {
	c := &Client{Run: newStub(map[string]string{
		"prlctl list -a --json": listSample,
	})}
	vms, err := c.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(vms) != 3 {
		t.Fatalf("expected 3 VMs, got %d", len(vms))
	}
	if vms[0].Name != "Ubuntu Server ARM64" || vms[0].Status != "suspended" {
		t.Errorf("unexpected first entry: %+v", vms[0])
	}
	// Find by name.
	got, err := c.Find(context.Background(), "Windows 11")
	if err != nil {
		t.Fatalf("Find: %v", err)
	}
	if got.Status != "stopped" {
		t.Errorf("expected stopped, got %s", got.Status)
	}
	// Find by UUID (case-insensitive).
	got, err = c.Find(context.Background(), "70C0420F-1717-42E3-85EE-B7F1FD33AC62")
	if err != nil {
		t.Fatalf("Find by uuid: %v", err)
	}
	if got.Name != "Ubuntu Server ARM64" {
		t.Errorf("uuid match wrong: %s", got.Name)
	}
	// Not found.
	if _, err := c.Find(context.Background(), "nope"); err == nil {
		t.Fatal("expected error for missing VM")
	}
}

const infoSample = `[{
  "ID": "d53f9ec7-88af-4baf-831a-08a68d4dbb9a",
  "Name": "Windows 11 Pro (Debugger)",
  "State": "running",
  "OS": "win-11",
  "Uptime": "226817",
  "Home path": "/Volumes/External/parallels/Win.pvm/config.pvs",
  "GuestTools": {"state": "installed", "version": "20.2.2-55879"},
  "Hardware": {
    "cpu": {"cpus": 4, "auto": "on"},
    "memory": {"size": "6144Mb", "auto": "on"},
    "hdd0": {"enabled": true, "image": "/v/Win-0.hdd", "type": "expanded", "size": "262144Mb"},
    "net0": {"enabled": true, "type": "shared", "mac": "001C42E10222", "card": "virtio"},
    "net1": {"enabled": true, "type": "bridged", "iface": "default", "mac": "001C42A12836", "card": "virtio"}
  },
  "Network": {
    "ipAddresses": [
      {"type": "ipv4", "ip": "10.211.55.5"},
      {"type": "ipv6", "ip": "fdb2:2c26:f4e4::1"},
      {"type": "ipv4", "ip": "192.168.15.244"}
    ]
  }
}]`

func TestInfo(t *testing.T) {
	c := &Client{Run: newStub(map[string]string{
		"prlctl list -i dbg --json": infoSample,
	})}
	info, err := c.Info(context.Background(), "dbg")
	if err != nil {
		t.Fatalf("Info: %v", err)
	}
	if info.State != "running" || info.OS != "win-11" {
		t.Errorf("state/os: %s/%s", info.State, info.OS)
	}
	if got := info.CPUs(); got != 4 {
		t.Errorf("CPUs: got %d want 4", got)
	}
	if got := info.MemoryMB(); got != 6144 {
		t.Errorf("MemoryMB: got %d want 6144", got)
	}
	if disks := info.Disks(); len(disks) != 1 || disks[0].Size != "262144Mb" {
		t.Errorf("Disks: %+v", disks)
	}
	if nets := info.Nets(); len(nets) != 2 || nets[1].Type != "bridged" {
		t.Errorf("Nets: %+v", nets)
	}
	if got := info.IPv4s(); len(got) != 2 || got[0] != "10.211.55.5" || got[1] != "192.168.15.244" {
		t.Errorf("IPv4s: %v", got)
	}
	if info.GuestTools.State != "installed" || info.GuestTools.Version != "20.2.2-55879" {
		t.Errorf("GuestTools: %+v", info.GuestTools)
	}
}

func TestSnapshotListEmpty(t *testing.T) {
	c := &Client{Run: newStub(map[string]string{
		"prlctl snapshot-list vm -j": "", // prlctl prints nothing when no snapshots
	})}
	snaps, err := c.SnapshotList(context.Background(), "vm")
	if err != nil {
		t.Fatalf("SnapshotList empty: %v", err)
	}
	if snaps != nil && len(snaps) != 0 {
		t.Fatalf("expected nil/empty, got %+v", snaps)
	}
}

const snapSample = `[
  {"id":"{a}","name":"base","date":"2026-03-01","current":"no","state":"stopped","description":"","parent":""},
  {"id":"{b}","name":"pre-experiment","date":"2026-03-21","current":"yes","state":"running","description":"before risky change","parent":"{a}"}
]`

func TestSnapshotList(t *testing.T) {
	c := &Client{Run: newStub(map[string]string{
		"prlctl snapshot-list vm -j": snapSample,
	})}
	snaps, err := c.SnapshotList(context.Background(), "vm")
	if err != nil {
		t.Fatalf("SnapshotList: %v", err)
	}
	if len(snaps) != 2 {
		t.Fatalf("expected 2 snapshots, got %d", len(snaps))
	}
	if !snaps[1].IsCurrent() || snaps[0].IsCurrent() {
		t.Errorf("current flag wrong: %+v", snaps)
	}
	if snaps[1].Description != "before risky change" {
		t.Errorf("description: %s", snaps[1].Description)
	}
}

func TestParseMegabytes(t *testing.T) {
	cases := map[string]int{
		"6144Mb":   6144,
		"262144Mb": 262144,
		"0Mb":      0,
		"auto":     -1,
		"":         -1,
		"garbage":  -1,
	}
	for in, want := range cases {
		if got := parseMegabytes(in); got != want {
			t.Errorf("parseMegabytes(%q) = %d, want %d", in, got, want)
		}
	}
}

func TestToGB(t *testing.T) {
	if got := toGB(64, "GB"); got != 64 {
		t.Errorf("toGB(64,GB)=%v", got)
	}
	if got := toGB(2, "TB"); got != 2048 {
		t.Errorf("toGB(2,TB)=%v", got)
	}
	if got := toGB(2048, "MB"); got != 2 {
		t.Errorf("toGB(2048,MB)=%v", got)
	}
}

func TestMemLabel(t *testing.T) {
	v, unit := memLabel("  Memory: 64 GB\n")
	if v != 64 || unit != "GB" {
		t.Errorf("memLabel: %v %q", v, unit)
	}
	if v, _ := memLabel("no match"); v != 0 {
		t.Errorf("memLabel no match: %v", v)
	}
}

func TestParseLoadAvg(t *testing.T) {
	got := parseLoadAvg("{ 0.10 0.20 0.30 }")
	if got == nil || got[0] != 0.1 || got[1] != 0.2 || got[2] != 0.3 {
		t.Errorf("parseLoadAvg: %v", got)
	}
	if parseLoadAvg("nada") != nil {
		t.Error("expected nil for garbage")
	}
}

func TestSnapshotIsCurrent(t *testing.T) {
	for _, c := range []string{"yes", "YES", "true", "1", "*"} {
		if !(Snapshot{Current: c}.IsCurrent()) {
			t.Errorf("%q should be current", c)
		}
	}
	for _, c := range []string{"no", "", "false", "0"} {
		if (Snapshot{Current: c}.IsCurrent()) {
			t.Errorf("%q should not be current", c)
		}
	}
}

func TestExitErrorNotFound(t *testing.T) {
	ee := &ExitError{Stderr: "The virtual machine could not be found."}
	if !ee.NotFound() {
		t.Error("expected NotFound")
	}
	ee2 := &ExitError{Stderr: "some other error"}
	if ee2.NotFound() {
		t.Error("unexpected NotFound")
	}
}

func TestInjectRootSSHKeyBuildsSafeScript(t *testing.T) {
	// Verify the injected script round-trips the key verbatim through base64,
	// so a key with shell-special characters cannot break quoting.
	var seen string
	c := &Client{Run: &captureRunner{on: func(bin string, args []string) {
		// args: exec <vm> /bin/sh -lc <script>
		seen = args[len(args)-1]
	}}}
	pub := "ssh-ed25519 AAAA '\"$(rm -rf /)\" comment"
	c.injectRootSSHKey(context.Background(), "vm", pub)
	// The script must contain the base64 of the key and NOT the raw key.
	if strings.Contains(seen, "rm -rf") {
		t.Fatalf("script contains raw dangerous key text: %q", seen)
	}
	b64 := strings.Split(seen, "echo ")[1]
	b64 = strings.Fields(b64)[0]
	decoded, err := base64.StdEncoding.DecodeString(b64)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	if string(decoded) != strings.TrimSpace(pub) {
		t.Errorf("round-trip mismatch: %q != %q", decoded, pub)
	}
}

// captureRunner captures the last invocation's args.
type captureRunner struct {
	on func(bin string, args []string)
}

func (r *captureRunner) Run(_ context.Context, bin string, args ...string) (*CmdResult, error) {
	r.on(bin, args)
	return &CmdResult{}, nil
}
