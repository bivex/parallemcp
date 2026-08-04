# parallemcp

A [Model Context Protocol](https://modelcontextprotocol.io) server that exposes
[Parallels Desktop](https://www.parallels.com/products/desktop/) VM management as
tools for LLM agents (Claude Code, Claude Desktop, …).

It wraps the `prlctl` / `prlsrvctl` command-line tools and speaks JSON-RPC over
stdin/stdout. No API keys, no network — it talks to the local Parallels service.

## Requirements

- macOS with **Parallels Desktop** installed (`prlctl` and `prlsrvctl` on `PATH`,
  e.g. `/usr/local/bin`). Developed against Parallels Desktop 20.2.2.
- Go 1.23+ (built/tested on 1.26).

## Build

```bash
git clone <this repo> parallemcp && cd parallemcp
go build -o parallemcp .
# sanity check: list the tools it exposes
./parallemcp   # then Ctrl-C; it reads JSON-RPC from stdin
```

Run the test suite:

```bash
go test ./...
```

## Configure

### Claude Code

```bash
claude mcp add parallemcp -- /absolute/path/to/parallemcp
```

### Claude Desktop (`claude_desktop_config.json`)

```jsonc
{
  "mcpServers": {
    "parallemcp": {
      "command": "/absolute/path/to/parallemcp"
    }
  }
}
```

## Tools (19)

**VM lifecycle**
| Tool | Description |
|---|---|
| `vm_list` | List all VMs with status and IP address |
| `vm_status` | Status of a specific VM (name or UUID) |
| `vm_info` | Full config: CPU, memory, disks, NICs, guest IPs |
| `vm_start` | Start a stopped/suspended VM |
| `vm_stop` | Graceful stop, or `force:true` to kill |
| `vm_restart` | Restart a running VM |
| `vm_suspend` | Suspend (save state to disk) |
| `vm_resume` | Resume a suspended VM |

**Snapshots**
| Tool | Description |
|---|---|
| `vm_snapshot_create` | Named snapshot, optional description |
| `vm_snapshot_list` | List snapshots for a VM |
| `vm_snapshot_restore` | Revert to a snapshot by `id` or `name` |

**Provisioning**
| Tool | Description |
|---|---|
| `vm_create` | New VM from a Parallels ostemplate (default Debian) |
| `vm_clone` | Linked or full clone |
| `vm_delete` | Delete VM + disks (`confirm:true` required) |
| `template_list` | Registered templates + base-image tokens |

**Operations**
| Tool | Description |
|---|---|
| `vm_exec` | Run a shell command inside a running VM |
| `vm_configure` | Change CPU / memory / name |
| `vm_shared_folders` | Manage host shared folders (list, add, remove) |
| `vm_file_copy` | Transfer files between host and guest VM (push/pull) |
| `vm_network` | Manage VM network adapters (list, set type/iface/status) |
| `vm_smb` | Manage Windows SMB shares and mapped network drives (list, share, mount, unmount) |
| `server_info` | Parallels version, license, host info |
| `host_stats` | macOS host CPU, memory, disk, load, uptime |

## Examples

- "List all my VMs" → `vm_list`
- "How much memory does the host have free?" → `host_stats`
- "Run `uptime` inside Windows 11 Pro (Debugger)" → `vm_exec {vm, command:"uptime"}`
- "Snapshot Ubuntu Server ARM64 before I do something risky" →
  `vm_snapshot_create {vm, name:"pre-experiment", description:"…"}`
- "Delete test-siem, I'm done with it" → `vm_delete {vm:"test-siem", confirm:true}`

## Notes

- **Stdout discipline.** The server communicates over stdin/stdout; all logging
  goes to **stderr** so it never corrupts the JSON-RPC stream.
- **`vm_create`** uses a Parallels *ostemplate* (`prlctl create -o debian -d debian`)
  rather than downloading a cloud image. It then sets CPU/memory, starts the VM,
  and **best-effort** injects your SSH public key into `/root/.ssh/authorized_keys`
  via `prlctl exec`. By default it auto-detects the first key under `~/.ssh`
  (`id_ed25519.pub` → `id_ecdsa.pub` → `id_rsa.pub`); pass `ssh_pubkey_path` to
  override. A VM created from an ostype without a registered template may lack an
  OS disk, in which case SSH injection / IP discovery fail gracefully and are
  reported as warnings rather than errors.
- **Destructive operations** (`vm_delete`, force `vm_stop`) require explicit
  confirmation.
- **`vm_exec`** runs the command with `/bin/sh -lc`, so pipes, redirection and
  `&&` work. The guest command's non-zero exit is surfaced as an exit code (not a
  tool error); only a failure to execute at all (e.g. Parallels Tools not running)
  is reported as an error.

## Layout

```
main.go            entry point: server + stdio transport
parallels/         pure CLI wrapper (prlctl/prlsrvctl), no MCP dependency
  runner.go          Runner interface + exec, ExitError
  client.go          Client with run/runJSON helpers
  types.go           VMInfo/Snapshot/ServerInfo/HostStats + accessors
  lifecycle.go       list/status/info/start/stop/restart/suspend/resume
  snapshot.go        snapshot create/list/restore
  provision.go       create/clone/delete/templates + SSH injection
  ops.go             exec/configure/server_info/host_stats
tools/             MCP layer: typed inputs + handlers + markdown output
  register.go        Register(server) + result helpers
  vm.go / snapshot.go / provision.go / ops.go
```
