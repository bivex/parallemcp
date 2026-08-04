package parallels

import (
	"encoding/json"
	"sort"
	"strconv"
	"strings"
)

// VMListEntry is one element of `prlctl list -a --json` (and `list -t --json`).
type VMListEntry struct {
	UUID         string `json:"uuid"`
	Status       string `json:"status"`
	IPConfigured string `json:"ip_configured"`
	Name         string `json:"name"`
}

// NetIP is a single guest IP address entry under VMInfo.Network.
type NetIP struct {
	Type string `json:"type"` // "ipv4" | "ipv6"
	IP   string `json:"ip"`
}

// VMInfo models the relevant fields of `prlctl list -i <vm> --json`. The Hardware
// object uses dynamic keys (cpu, memory, hdd0, net0, ...) so it is decoded as a
// raw map and accessed via the typed helpers below.
type VMInfo struct {
	ID         string `json:"ID"`
	Name       string `json:"Name"`
	State      string `json:"State"`
	OS         string `json:"OS"`
	Template   string `json:"Template"`
	Uptime     string `json:"Uptime"`
	HomePath   string `json:"Home path"`
	GuestTools struct {
		State   string `json:"state"`
		Version string `json:"version"`
	} `json:"GuestTools"`
	Hardware map[string]json.RawMessage `json:"Hardware"`
	Network  struct {
		IPAddresses []NetIP `json:"ipAddresses"`
	} `json:"Network"`
}

// hwCPU is the cpu entry within Hardware.
type hwCPU struct {
	CPUs int `json:"cpus"`
}

// hwMem is the memory entry within Hardware (size like "6144Mb" or "auto").
type hwMem struct {
	Size string `json:"size"`
}

// HwDisk is one hdd* entry within Hardware.
type HwDisk struct {
	Enabled bool   `json:"enabled"`
	Image   string `json:"image"`
	Type    string `json:"type"`
	Size    string `json:"size"` // e.g. "262144Mb"
}

// HwNet is one net* entry within Hardware.
type HwNet struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"` // shared | bridged | host-only
	MAC     string `json:"mac"`
	Card    string `json:"card"`
	Iface   string `json:"iface"` // bridged: host interface
}

// CPUs returns the configured vCPU count, or 0 if unavailable.
func (v *VMInfo) CPUs() int {
	raw, ok := v.Hardware["cpu"]
	if !ok {
		return 0
	}
	var c hwCPU
	_ = json.Unmarshal(raw, &c)
	return c.CPUs
}

// MemoryMB returns configured memory in megabytes, or -1 if unavailable.
func (v *VMInfo) MemoryMB() int {
	raw, ok := v.Hardware["memory"]
	if !ok {
		return -1
	}
	var m hwMem
	if err := json.Unmarshal(raw, &m); err != nil {
		return -1
	}
	return parseMegabytes(m.Size)
}

// Disks returns all hdd* entries (enabled and disabled), in key order.
func (v *VMInfo) Disks() []HwDisk {
	var keys []string
	for k := range v.Hardware {
		if strings.HasPrefix(k, "hdd") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []HwDisk
	for _, k := range keys {
		var d HwDisk
		if err := json.Unmarshal(v.Hardware[k], &d); err == nil {
			out = append(out, d)
		}
	}
	return out
}

// Nets returns all net* entries, in key order.
func (v *VMInfo) Nets() []HwNet {
	var keys []string
	for k := range v.Hardware {
		if strings.HasPrefix(k, "net") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []HwNet
	for _, k := range keys {
		var n HwNet
		if err := json.Unmarshal(v.Hardware[k], &n); err == nil {
			out = append(out, n)
		}
	}
	return out
}

// IPv4s returns the guest IPv4 addresses reported by Parallels Tools.
func (v *VMInfo) IPv4s() []string {
	var out []string
	for _, a := range v.Network.IPAddresses {
		if a.Type == "ipv4" && a.IP != "" {
			out = append(out, a.IP)
		}
	}
	return out
}

// Snapshot models one element of `prlctl snapshot-list <vm> -j`.
type Snapshot struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Date        string `json:"date"`
	Current     string `json:"current"` // "yes"/"no" (or "true"/"false")
	State       string `json:"state"`
	Description string `json:"description"`
	Parent      string `json:"parent"`
}

// IsCurrent reports whether this is the active snapshot.
func (s Snapshot) IsCurrent() bool {
	switch strings.ToLower(s.Current) {
	case "yes", "true", "1", "*":
		return true
	}
	return false
}

// ServerInfo models the relevant fields of `prlsrvctl info --json`.
type ServerInfo struct {
	ID       string `json:"ID"`
	Hostname string `json:"Hostname"`
	OS       string `json:"OS"`
	Version  string `json:"Version"`
	VMHome   string `json:"VM home"`
	License  struct {
		State      string `json:"state"`
		Restricted string `json:"restricted"`
	} `json:"License"`
}

// HostStats holds macOS host statistics parsed from system_profiler /
// vm_stat / df / sysctl. Fields that could not be parsed are left zero/empty.
type HostStats struct {
	Chip          string
	PhysicalCores int
	LogicalCores  int
	MemoryTotalGB float64
	MemoryFreeGB  float64
	DiskUsedGB    float64
	DiskTotalGB   float64
	LoadAvg1      float64
	LoadAvg5      float64
	LoadAvg15     float64
	UptimeDays    float64
}

// parseMegabytes turns Parallels size strings ("6144Mb", "auto") into an int MB,
// returning -1 when it cannot be parsed.
func parseMegabytes(s string) int {
	s = strings.TrimSpace(strings.ToLower(s))
	if s == "" || s == "auto" {
		return -1
	}
	n, err := strconv.Atoi(strings.TrimSuffix(s, "mb"))
	if err != nil {
		return -1
	}
	return n
}
