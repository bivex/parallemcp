# Native Interactive Kernel Debugging Guide via MCP (`parallemcp`)

Complete guide for performing **live kernel debugging, ARM64 register inspection, instruction disassembly, single-stepping, and serial console interaction** directly through **MCP tools** without writing Python scripts or manual terminal commands.

---

## 🏗️ Interactive Debugging Workflow

```
┌───────────────────────────────────────────────────────────┐
│                    LLM Agent / User                       │
│                                                           │
│ 1. vm_configure_kernel_debug  ──> Configure Debug Flags   │
│ 2. vm_start                   ──> Boot Virtual Machine    │
│ 3. vm_debug_registers         ──> Read PC, SP, CPSR       │
│ 4. vm_debug_disassemble       ──> Read ARM64 Assembly     │
│ 5. vm_debug_step              ──> Single-Step (stepi)     │
│ 6. vm_debug_serial            ──> TTY Serial Console IO   │
└────────────────────────────┬──────────────────────────────┘
                             │
                             │ Native MCP Tool Calls
                             ▼
┌───────────────────────────────────────────────────────────┐
│               Parallels Desktop (macOS host)              │
│                                                           │
│  - Parallels GDB Stub (127.0.0.1:<port>)                  │
│  - Serial COM Socket (/Volumes/.../kd.sock)               │
│  - Linux Kernel 6.6 ARM64 (EL1 Mode, PAN Active)          │
└───────────────────────────────────────────────────────────┘
```

---

## 🛠️ Step 1: Automated Debugger Setup (`vm_configure_kernel_debug`)

Configure the VM's hardware and debugger settings in a single tool call before starting the VM.

### Tool Call:
```json
{
  "vm": "alpine-test",
  "mode": "gdb",
  "port": 50000
}
```

### Result:
- Writes `vm.debug=1&vm.debug.protocol=0&vm.debug.local_addr=127.0.0.1` to VM configuration.
- Automatically creates a user-owned Unix socket (`<HomePath>/kd.sock`) inside the `.pvm` directory to avoid permission issues.

---

## ⚡ Step 2: Start VM (`vm_start`) & Discover Port

Power on the VM:

```json
{
  "vm": "alpine-test"
}
```

Check the active listening GDB TCP port using `vm_info` or system status. Parallels GDB stub binds to a local port (e.g. `127.0.0.1:50678`).

---

## 📊 Step 3: Inspect CPU Registers (`vm_debug_registers`)

Read all 31 ARM64 general-purpose registers (`X0`-`X30`), Stack Pointer (`SP`), Program Counter (`PC`), and Current Program Status Register (`CPSR`) natively.

### Tool Call:
```json
{
  "target": "127.0.0.1:50678",
  "arch": "aarch64"
}
```

### Response Example:
```gdb
The target architecture is set to "aarch64".
0xffff800080b554cc in ?? ()
x0             0x0                 0
x1             0x0                 0
x19            0xffff8000813d9008  -140735320059896
x29            0xffff8000815d3d60  -140735317983904
x30            0xffff800080b554e0  -140735328987936
sp             0xffff8000815d3d60  0xffff8000815d3d60
pc             0xffff800080b554cc  0xffff800080b554cc
cpsr           0x614000c5          [ SP EL=1 F I BTYPE=0 PAN DIT C Z ]
```

### Key Security Flags to Inspect:
- **`SP EL=1`**: Execution level is **Exception Level 1 (Kernel Mode)**.
- **`PAN`**: Hardware **Privileged Access Never** protection is enabled.

---

## 🔍 Step 4: Disassemble Memory (`vm_debug_disassemble`)

Disassemble ARM64 instructions around the current `PC` or target address.

### Tool Call:
```json
{
  "target": "127.0.0.1:50678",
  "address": "0xffff800080b554cc",
  "count": 10,
  "arch": "aarch64"
}
```

### Response Example:
```assembly
Dump of assembler code from 0xffff800080b554cc to 0xffff800080b554f4:
=> 0xffff800080b554cc:	ret
   0xffff800080b554d0:	paciasp
   0xffff800080b554d4:	stp	x29, x30, [sp, #-16]!
   0xffff800080b554d8:	mov	x29, sp
   0xffff800080b554dc:	bl	0xffff800080b554c4
   0xffff800080b554e0:	ldp	x29, x30, [sp], #16
   0xffff800080b554e4:	autiasp
   0xffff800080b554e8:	mov	x16, #0x0
   0xffff800080b554ec:	mov	x17, #0x0
   0xffff800080b554f0:	ret
End of assembler dump.
```

---

## 👣 Step 5: Single-Instruction Stepping (`vm_debug_step`)

Execute single instruction steps (`stepi`) and observe changing registers and instruction pointers.

### Tool Call:
```json
{
  "target": "127.0.0.1:50678",
  "steps": 1,
  "arch": "aarch64"
}
```

### Response Example:
```gdb
0xffff800080b554e0 in ?? ()
pc             0xffff800080b554e0  0xffff800080b554e0
sp             0xffff8000815d3d60  0xffff8000815d3d60
cpsr           0x614000c5          [ SP EL=1 F I BTYPE=0 PAN DIT C Z ]
```

---

## 💬 Step 6: Serial Console IO (`vm_debug_serial`)

Interact with the VM's TTY serial console directly over the Unix socket (`kd.sock`) without writing custom Python scripts.

### Read Serial Output:
```json
{
  "socket_path": "/Volumes/External/parallels/alpine-test.pvm/kd.sock"
}
```

### Send Command to Guest Serial Shell:
```json
{
  "socket_path": "/Volumes/External/parallels/alpine-test.pvm/kd.sock",
  "send_string": "uname -a\n"
}
```

### Response Example:
```text
## Serial Socket Output: `/Volumes/External/parallels/alpine-test.pvm/kd.sock`

Linux alpine-test 6.6.52-0-virt #1-Alpine SMP PREEMPT_DYNAMIC aarch64 GNU/Linux
```

---

## 💡 Summary of Tools Used

| Tool | Category | Primary Function |
|---|---|---|
| `vm_configure_kernel_debug` | Setup | Configures KDNET, Serial COM socket, and Parallels GDB stub |
| `vm_start` | Lifecycle | Powers on the VM |
| `vm_debug_registers` | Inspection | Reads `PC`, `SP`, `CPSR`, `X0-X30` register state |
| `vm_debug_disassemble` | Inspection | Disassembles ARM64/x86_64 instructions around `PC` |
| `vm_debug_step` | Execution | Performs single-instruction step (`stepi`) |
| `vm_debug_serial` | I/O | Reads/writes TTY serial console data directly over Unix socket |
