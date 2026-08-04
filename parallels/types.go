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

// VMInfo models the relevant fields of `prlctl list -i <vm> --json`.
type VMInfo struct {
	ID            string `json:"ID"`
	Name          string `json:"Name"`
	Description   string `json:"Description"`
	State         string `json:"State"`
	OS            string `json:"OS"`
	Template      string `json:"Template"`
	Uptime        string `json:"Uptime"`
	HomePath      string `json:"Home path"`
	BootOrder     string `json:"Boot order"`
	BIOSType      string `json:"BIOS type"`
	EFISecureBoot string `json:"EFI Secure boot"`
	SMBIOS        struct {
		BIOSVersion  string `json:"BIOS Version"`
		SerialNumber string `json:"System serial number"`
		Manufacturer string `json:"Board Manufacturer"`
	} `json:"SMBIOS settings"`
	GuestTools struct {
		State   string `json:"state"`
		Version string `json:"version"`
	} `json:"GuestTools"`
	Security struct {
		Encrypted     string `json:"Encrypted"`
		TPMEnabled    string `json:"TPM enabled"`
		TPMType       string `json:"TPM type"`
		Protected     string `json:"Protected"`
		Archived      string `json:"Archived"`
		Packed        string `json:"Packed"`
		PassProtected string `json:"Custom password protection"`
		Locked        string `json:"Configuration is locked"`
	} `json:"Security"`
	Optimization struct {
		FasterVM        string `json:"Faster virtual machine"`
		HypervisorType  string `json:"Hypervisor type"`
		AdaptiveHV      string `json:"Adaptive hypervisor"`
		NestedVirt      string `json:"Nested virtualization"`
		PMUVirt         string `json:"PMU virtualization"`
		AutoCompress    string `json:"Auto compress virtual disks"`
		ResourceQuota   string `json:"Resource quota"`
	} `json:"Optimization"`
	StartupShutdown struct {
		Autostart    string `json:"Autostart"`
		Autostop     string `json:"Autostop"`
		OnShutdown   string `json:"On shutdown"`
		OnWindowClose string `json:"On window close"`
		PauseIdle    string `json:"Pause idle"`
		UndoDisks    string `json:"Undo disks"`
	} `json:"Startup and Shutdown"`
	TimeSyncronization struct {
		Enabled  bool   `json:"enabled"`
		Interval int    `json:"Interval (in seconds)"`
		SmartMode string `json:"Smart mode"`
	} `json:"Time Synchronization"`
	SmartGuard struct {
		Enabled bool `json:"enabled"`
	} `json:"Smart Guard"`
	USBBluetooth struct {
		USB30          string `json:"Support USB 3.0"`
		ShareCameras   string `json:"Automatic sharing cameras"`
		ShareBluetooth string `json:"Automatic sharing bluetooth"`
		ShareGamepads  string `json:"Automatic sharing gamepads"`
	} `json:"USB and Bluetooth"`
	MouseKeyboard struct {
		SmartMouse      string `json:"Smart mouse optimized for games"`
		StickyMouse     string `json:"Sticky mouse"`
		SmoothScrolling string `json:"Smooth scrolling"`
		KeyboardMode    string `json:"Keyboard optimization mode"`
	} `json:"Mouse and Keyboard"`
	PrintManagement struct {
		SyncPrinters string `json:"Synchronize with host printers"`
		SyncDefault  string `json:"Synchronize default printer"`
	} `json:"Print Management"`
	TravelMode struct {
		EnterCondition string `json:"Enter condition"`
		Threshold      int    `json:"Enter threshold"`
	} `json:"Travel mode"`
	SharedProfile struct {
		Enabled      bool   `json:"enabled"`
		UseDesktop   string `json:"Use desktop"`
		UseDocuments string `json:"Use documents"`
		UseDownloads string `json:"Use downloads"`
		UsePictures  string `json:"Use pictures"`
		UseMusic     string `json:"Use music"`
		UseMovies    string `json:"Use movies"`
	} `json:"Shared Profile"`
	SharedApps struct {
		Enabled     bool   `json:"enabled"`
		HostToGuest string `json:"Host-to-guest apps sharing"`
		GuestToHost string `json:"Guest-to-host apps sharing"`
		DockFolder  string `json:"Show guest apps folder in Dock"`
	} `json:"Shared Applications"`
	SmartMount struct {
		Enabled         bool   `json:"enabled"`
		RemovableDrives string `json:"Removable drives"`
		CDDVDDrives     string `json:"CD/DVD drives"`
		NetworkShares   string `json:"Network shares"`
	} `json:"SmartMount"`
	MiscSharing struct {
		SharedClipboardMode string `json:"Shared clipboard mode"`
		SharedCloud         string `json:"Shared cloud"`
	} `json:"Miscellaneous Sharing"`
	Advanced struct {
		HostnameSync   string `json:"VM hostname synchronization"`
		SSHKeysSync    string `json:"Public SSH keys synchronization"`
		DeveloperTools string `json:"Show developer tools"`
		RosettaLinux   string `json:"Rosetta Linux"`
		ShareLocation  string `json:"Share host location"`
	} `json:"Advanced"`
	Network struct {
		Conditioned string  `json:"Conditioned"`
		IPAddresses []NetIP `json:"ipAddresses"`
	} `json:"Network"`
	SharedFolderSettings struct {
		Enabled   bool   `json:"enabled"`
		Automount string `json:"Automount"`
	} `json:"Guest Shared Folders"`
	Hardware             map[string]json.RawMessage `json:"Hardware"`
	HostSharedFoldersRaw map[string]json.RawMessage `json:"Host Shared Folders"`
}

// hwCPU is the cpu entry within Hardware.
type hwCPU struct {
	CPUs int `json:"cpus"`
}

// hwMem is the memory entry within Hardware (size like "6144Mb" or "auto").
type hwMem struct {
	Size string `json:"size"`
}

// hwCPUFull is the full cpu entry within Hardware.
type hwCPUFull struct {
	CPUs  int    `json:"cpus"`
	Auto  string `json:"auto"`
	VTx   bool   `json:"VT-x"`
	Accl  string `json:"accl"`
	Mode  string `json:"mode"`
	Type  string `json:"type"` // arm | x86
}

// CPUDetails returns extended CPU info (arch, accel, VT-x).
func (v *VMInfo) CPUDetails() *hwCPUFull {
	raw, ok := v.Hardware["cpu"]
	if !ok {
		return nil
	}
	var c hwCPUFull
	if err := json.Unmarshal(raw, &c); err != nil {
		return nil
	}
	return &c
}

// HwDisk is one hdd* entry within Hardware.
type HwDisk struct {
	Enabled       bool   `json:"enabled"`
	Port          string `json:"port"`
	Image         string `json:"image"`
	Type          string `json:"type"`
	Size          string `json:"size"` // e.g. "262144Mb"
	OnlineCompact string `json:"online-compact"` // "on" | "off"
}

// HwCDROM is one cdrom* entry within Hardware.
type HwCDROM struct {
	Enabled bool   `json:"enabled"`
	Port    string `json:"port"`
	Image   string `json:"image"`
	State   string `json:"state"` // connected | disconnected
}

// CDROMs returns all cdrom* entries, in key order.
func (v *VMInfo) CDROMs() []HwCDROM {
	var keys []string
	for k := range v.Hardware {
		if strings.HasPrefix(k, "cdrom") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []HwCDROM
	for _, k := range keys {
		var c HwCDROM
		if err := json.Unmarshal(v.Hardware[k], &c); err == nil {
			out = append(out, c)
		}
	}
	return out
}

// HwSerial is one serial* entry within Hardware.
type HwSerial struct {
	Enabled bool   `json:"enabled"`
	Socket  string `json:"socket"`
	Mode    string `json:"mode"` // server | client
}

// Serials returns all serial* entries, in key order.
func (v *VMInfo) Serials() []HwSerial {
	var keys []string
	for k := range v.Hardware {
		if strings.HasPrefix(k, "serial") {
			keys = append(keys, k)
		}
	}
	sort.Strings(keys)
	var out []HwSerial
	for _, k := range keys {
		var s HwSerial
		if err := json.Unmarshal(v.Hardware[k], &s); err == nil {
			out = append(out, s)
		}
	}
	return out
}

// HwSound is the sound* entry within Hardware.
type HwSound struct {
	Enabled bool   `json:"enabled"`
	Output  string `json:"output"`
	Mixer   string `json:"mixer"`
}

// Sound returns the first sound adapter, or nil.
func (v *VMInfo) Sound() *HwSound {
	raw, ok := v.Hardware["sound0"]
	if !ok {
		return nil
	}
	var s HwSound
	if err := json.Unmarshal(raw, &s); err != nil {
		return nil
	}
	return &s
}

// HwNet is one net* entry within Hardware.
type HwNet struct {
	Enabled bool   `json:"enabled"`
	Type    string `json:"type"` // shared | bridged | host-only
	MAC     string `json:"mac"`
	Card    string `json:"card"`
	Iface   string `json:"iface"` // bridged: host interface
}

// HwVideo is the video entry within Hardware.
type HwVideo struct {
	AdapterType    string `json:"adapter-type"`
	Size           string `json:"size"`
	ThreeDAccel    string `json:"3d-acceleration"`
	HighResolution string `json:"high-resolution"`
	AutoMem        string `json:"automatic-video-memory"`
}

// Video returns the video adapter settings, or nil if not found.
func (v *VMInfo) Video() *HwVideo {
	raw, ok := v.Hardware["video"]
	if !ok {
		return nil
	}
	var vid HwVideo
	if err := json.Unmarshal(raw, &vid); err != nil {
		return nil
	}
	return &vid
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

// SharedFolder models a host shared folder entry under Host Shared Folders.
type SharedFolder struct {
	Name    string `json:"name"`
	Path    string `json:"path"`
	Mode    string `json:"mode"`
	Enabled bool   `json:"enabled"`
}

type rawSharedFolder struct {
	Enabled bool   `json:"enabled"`
	Path    string `json:"path"`
	Mode    string `json:"mode"`
}

// SharedFolders returns all host shared folders defined for this VM.
func (v *VMInfo) SharedFolders() []SharedFolder {
	if len(v.HostSharedFoldersRaw) == 0 {
		return nil
	}
	var keys []string
	for k := range v.HostSharedFoldersRaw {
		if k == "enabled" {
			continue
		}
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var out []SharedFolder
	for _, k := range keys {
		var sf rawSharedFolder
		if err := json.Unmarshal(v.HostSharedFoldersRaw[k], &sf); err == nil {
			out = append(out, SharedFolder{
				Name:    k,
				Path:    sf.Path,
				Mode:    sf.Mode,
				Enabled: sf.Enabled,
			})
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
