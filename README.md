# Parallels MCP Server (`parallemcp`)

[![Go Version](https://img.shields.io/badge/Go-1.23%2B-00ADD8?style=flat-square&logo=go)](https://golang.org)
[![MCP Protocol](https://img.shields.io/badge/MCP-1.0.0-purple?style=flat-square)](https://modelcontextprotocol.io)
[![Parallels Desktop](https://img.shields.io/badge/Parallels_Desktop-20.x-red?style=flat-square)](https://www.parallels.com/products/desktop/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg?style=flat-square)](LICENSE)

A high-performance, native [Model Context Protocol (MCP)](https://modelcontextprotocol.io) server that empowers LLM agents (**Claude Code**, **Claude Desktop**, **Antigravity**, **Cursor**) to seamlessly manage, inspect, control, and automate **Parallels Desktop** virtual machines on macOS.

---

## ⚡ Key Highlights

- **40 Native MCP Tools:** Comprehensive suite covering VM lifecycle, snapshots, execution, file transfers, networking, SMB shares, hardware devices, interactive kernel debugging, breakpoints, live GDB stepping, disk compacting, and real-time PNG screenshots.
- **Automated Kernel Debugging & Live Breakpoints:** Automated setup for Windows KDNET, Serial COM sockets (`/tmp/` or `.pvm` bundle dir), and Parallels GDB stub interfaces. Native tools for reading CPU registers (`vm_debug_registers`), single-stepping instructions (`vm_debug_step`), disassembling code (`vm_debug_disassemble`), setting software & hardware breakpoints (`vm_debug_breakpoint_set`, `vm_debug_breakpoint_delete`, `vm_debug_breakpoint_list`, `vm_debug_continue`), and serial socket IO (`vm_debug_serial`) without custom scripts.
- **Zero Third-Party Dependencies / Network Overhead:** Wraps native `prlctl` and `prlsrvctl` CLI tools over JSON-RPC over stdin/stdout with strict stderr logging discipline.
- **Smart Windows & Linux Support:**
  - **Auto-PowerShell Detection:** Automatically detects PowerShell cmdlets (`Get-`, `Set-`, `$`, `[System.IO]`) and routes execution through clean, non-interactive PowerShell environments.
  - **Chunked Base64 File Transfer:** Zero-dependency, chunked file transfer (`vm_file_copy`) handling multi-megabyte payloads without Windows CMD character length limits.
- **Live Display Screenshots:** Capture full-resolution PNG screenshots of running VMs (`vm_screenshot`) for visual inspection and multimodal vision AI analysis.
- **Resilient Fallbacks:** Intelligent fallback mechanisms for OS creation, graceful stop timeouts (`--kill`), and cross-platform path parsing.

---

## 🛠️ Complete Tools Reference (40 Tools)

### 📊 1. System & Host Information (4 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `server_info` | — | View Parallels Desktop version, build, license status, and host system info. |
| `host_stats` | — | Monitor macOS host CPU usage, RAM utilization, disk space, load average, and uptime. |
| `template_list` | — | List registered templates and base-image distribution tokens. |
| `vm_list` | — | List all VMs with UUIDs, power status, and configured IP addresses. |

### ⚙️ 2. VM Lifecycle & Power Management (7 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `vm_status` | `vm` | Check detailed power status and IP configuration of a specific VM. |
| `vm_info` | `vm` | View full hardware profile: vCPUs, RAM, virtual disks, network cards, and guest info. |
| `vm_start` | `vm` | Power on a stopped or suspended VM. |
| `vm_stop` | `vm`, `force?` | Gracefully shut down a VM (with automatic `--kill` fallback if ACPI hangs). |
| `vm_suspend` | `vm` | Suspend VM and save execution state to disk. |
| `vm_resume` | `vm` | Resume a suspended VM. |
| `vm_restart` | `vm` | Reboot a running VM. |

### 🐛 3. Kernel Debugging & Breakpoint Control (11 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `vm_configure_kernel_debug` | `vm`, `mode?`, `port?`, `key?`, `host_ip?`, `socket_path?`, `auto_bcdedit?` | Automate kernel debugging setup (Windows KDNET, Serial COM Socket, or GDB stub). Auto-runs `bcdedit` in Windows guests. |
| `vm_configure_debugger` | `vm`, `enable`, `protocol?`, `local_addr?` | Enable/disable built-in Parallels guest debugger (`vm.debug` flags) for GDB or WinDbg. |
| `vm_guest_debugger` | `vm`, `port?` | Attach built-in Parallels guest debugger to a running VM and return host connection port. |
| `vm_debug_registers` | `target`, `arch?` | Read CPU registers (PC, SP, CPSR, X0-X30) directly from GDB target without custom scripts. |
| `vm_debug_step` | `target`, `arch?`, `steps?` | Execute single-instruction step (`stepi`) and return updated registers & disassembly. |
| `vm_debug_disassemble` | `target`, `address?`, `count?`, `arch?` | Disassemble instructions around PC or target address. |
| `vm_debug_breakpoint_set` | `target`, `location`, `hardware?`, `arch?` | Set a software (`break`) or hardware (`hbreak`) breakpoint at function name or memory address. |
| `vm_debug_breakpoint_delete` | `target`, `number?`, `arch?` | Delete a breakpoint by number (or delete all breakpoints). |
| `vm_debug_breakpoint_list` | `target`, `arch?` | List all active breakpoints on GDB target. |
| `vm_debug_continue` | `target`, `arch?` | Resume VM execution on GDB target until a breakpoint is hit or process stops. |
| `vm_debug_serial` | `socket_path`, `send_string?`, `timeout_sec?` | Read or write data directly over a VM's serial COM Unix socket (`kd.sock`). |



### 🛠️ 4. Provisioning, Resources & Bundles (5 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `vm_configure` | `vm`, `cpus?`, `memory_mb?`, `name?` | Reconfigure CPU count, RAM size, or VM name. |
| `vm_create` | `name`, `cpus?`, `memory_mb?`, `distribution?`, `ssh_pubkey_path?` | Create, configure, start a new VM and inject root SSH keys. |
| `vm_clone` | `source`, `name`, `linked?` | Perform a full or linked clone of an existing VM. |
| `vm_delete` | `vm`, `confirm: true` | Safely delete a VM and its disk files (auto-stops VM if running). |
| `vm_bundle` | `action` (`register`/`unregister`), `path?`, `vm?` | Register existing `.pvm` bundles or unregister without deleting files. |

### 📸 5. Snapshots & Visual Display (5 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `vm_snapshot_create` | `vm`, `name`, `description?` | Take a named snapshot of a VM. |
| `vm_snapshot_list` | `vm` | View full snapshot tree (UUIDs, names, dates, and active flags). |
| `vm_snapshot_restore` | `vm`, `id?`, `name?` | Revert VM state to a specific snapshot by ID or name. |
| `vm_snapshot_delete` | `vm`, `id` | Delete a snapshot to free disk space. |
| `vm_screenshot` | `vm`, `file_path?` | Capture a high-resolution PNG screenshot of a VM's display. |

### 💻 6. Operations, Guest Execution & Devices (8 Tools)

| Tool | Parameters | Description |
|---|---|---|
| `vm_exec` | `vm`, `command` | Execute shell / CMD / PowerShell commands inside a running guest OS. |
| `vm_file_copy` | `vm`, `direction`, `host_path`, `guest_path` | Transfer files bidirectional (`to_guest`, `from_guest`) with base64 chunking. |
| `vm_shared_folders` | `vm`, `action`, `name?`, `path?` | Manage host shared folders (`list`, `add`, `remove`). |
| `vm_smb` | `vm`, `action`, `share_name?`, `folder_path?`, `remote_ip?`, `drive_letter?` | Manage Windows SMB shares and mapped network drives (`list`, `share`, `mount`, `unmount`). |
| `vm_network` | `vm`, `action`, `type?`, `iface?`, `status?` | Configure network adapters (`shared`, `bridged`, `host-only`). |
| `vm_cdrom` | `vm`, `action`, `image_path?`, `device?` | Mount, connect, disconnect, or eject ISO images in CD/DVD drives. |
| `vm_compact` | `vm` | Enable online compacting and shrink `.hdd` virtual disks to reclaim Mac disk space. |
| `vm_disk_manage` | `vm`, `action`, `size`, `device?` | Add a new virtual hard disk or resize an existing disk (`add`, `resize`). |
| `vm_install_tools` | `vm` | Mount Parallels Guest Tools installer ISO into the guest OS. |
| `vm_device` | `vm`, `action`, `device_type?`, `device_name?` | Add or remove hardware devices (network adapters, serial ports, sound cards). |


---

## 💻 Requirements & Installation

### Requirements

- **macOS** with **Parallels Desktop 19 / 20+** installed (`prlctl` and `prlsrvctl` on `PATH`, e.g. `/usr/local/bin`).
- **Go 1.23+** for compilation.

### Build from Source

```bash
git clone https://github.com/bivex/parallemcp.git
cd parallemcp
go build -o parallemcp .
```

### Run Tests

```bash
go test -v ./...
```

---

## ⚙️ Client Configuration

### Claude Code CLI

```bash
claude mcp add parallels -- /absolute/path/to/parallemcp
```

### Claude Desktop (`claude_desktop_config.json`)

```json
{
  "mcpServers": {
    "parallels": {
      "command": "/absolute/path/to/parallemcp"
    }
  }
}
```

### Antigravity / Cursor / VSCode MCP Config

```json
{
  "mcpServers": {
    "parallels": {
      "command": "/absolute/path/to/parallemcp",
      "args": []
    }
  }
}
```

---

## 💡 Example Prompt Scenarios

- **Inspect Infrastructure:** *"List all running virtual machines and check macOS host CPU/RAM status."*
- **Visual Status Check:** *"Take a screenshot of Windows 11 Pro (Debugger) and tell me what is currently displayed."*
- **File Transfer:** *"Copy `/tmp/config.json` on my Mac to `C:\App\config.json` inside Windows 11."*
- **Windows SMB Share Setup:** *"Share `C:\Projects` as `ProjectsShare` on Windows 11 and mount it as drive `S:` on my second Windows VM."*
- **Snapshot & Experiment:** *"Create a snapshot named `pre-update` on Ubuntu Server ARM64 before running system updates."*
- **Disk Space Optimization:** *"Compact virtual disks for all stopped Linux VMs to free up Mac storage."*

---

## 🏗️ Architecture & Layout

```
parallemcp/
├── main.go               # Entry point: Server initialization & stdio JSON-RPC transport
├── parallels/            # Core Parallels CLI engine (prlctl / prlsrvctl)
│   ├── client.go         # Command execution engine & JSON parsing
│   ├── types.go          # Core models (VMInfo, VMListEntry, Snapshot, HostStats)
│   ├── lifecycle.go      # List, status, info, power controls (start, stop, suspend, resume)
│   ├── ops.go            # vm_exec engine, PowerShell auto-detection, vm_configure
│   ├── file_copy.go      # Chunked Base64 stream file transfers (to_guest / from_guest)
│   ├── smb.go            # Windows SMB shares & network drive mapping
│   ├── snapshot.go       # Snapshot management (create, list, switch, delete)
│   ├── provision.go      # VM creation, cloning, deletion, SSH key injection
│   └── extra_ops.go      # Screenshots, disk compacting, CDROM ISOs, hardware devices
└── tools/                # MCP Protocol Layer
    ├── register.go       # Tool registration & response formatting
    ├── vm.go             # Lifecycle & info tool handlers
    ├── ops.go            # Exec & configure tool handlers
    ├── file_copy.go      # File transfer tool handler
    ├── network.go        # Network adapter tool handler
    ├── smb.go            # SMB share tool handler
    ├── shared_folders.go # Shared folders tool handler
    ├── snapshot.go       # Snapshot tool handlers
    ├── provision.go      # Provisioning tool handlers
    └── extra_tools.go    # Screenshot, compact, CDROM, device & bundle tool handlers
```

---

## 📄 License

This project is open-source software licensed under the [MIT License](LICENSE).
