# Linux Kernel Module Debugging Guide (ARM64 / Parallels Desktop / macOS)

Guide for writing, building, and debugging a custom **Linux Kernel Module (LKM)** on Apple Silicon ARM64 using **Parallels Desktop**, **GDB**, and **Parallemcp MCP Server**.

---

## 📋 System Architecture

```
┌───────────────────────────────────────────────────────────┐
│                    macOS Host (ARM64)                     │
│                                                           │
│  ┌────────────────────┐          ┌─────────────────────┐  │
│  │ GDB 17.2 (aarch64) │ ───────> │ Parallemcp MCP      │  │
│  └─────────┬──────────┘          └─────────────────────┘  │
│            │ RSP Packet Protocol                          │
│            │ (127.0.0.1:49930)                            │
└────────────┼──────────────────────────────────────────────┘
             │
┌────────────▼──────────────────────────────────────────────┐
│            Parallels Hypervisor (Apple Silicon)           │
│  ┌─────────────────────────────────────────────────────┐  │
│  │             Alpine Linux ARM64 (Kernel 6.6)          │  │
│  │  - EL1 (Kernel Mode, PAN Enabled)                   │  │
│  │  - demo_mod.ko (Custom Kernel Driver)               │  │
│  └─────────────────────────────────────────────────────┘  │
└───────────────────────────────────────────────────────────┘
```

---

## 🛠️ 1. Configuring VM Debugger via Parallemcp MCP

Before starting the VM, enable the Parallels GDB stub interface via `vm_configure_kernel_debug` or `vm_configure_debugger`:

### MCP Tool Command:
```json
{
  "vm": "alpine-test",
  "mode": "gdb",
  "port": 50000
}
```

This writes system flags `vm.debug=1&vm.debug.protocol=0&vm.debug.local_addr=127.0.0.1` to `config.pvs`.

---

## 📝 2. Demo Driver Source Code (`demo_mod.c`)

Create `demo_mod.c`:

```c
#include <linux/init.h>
#include <linux/module.h>
#include <linux/kernel.h>

MODULE_LICENSE("GPL");
MODULE_AUTHOR("Antigravity");
MODULE_DESCRIPTION("Demo LKM for ARM64 GDB Kernel Debugging");

// Target function for GDB Breakpoint testing
noinline void demo_target_func(void) {
    pr_info("[DEMO_LKM] ====> Inside demo_target_func()! Breakpoint hit successfully!\n");
}

static int __init demo_init(void) {
    pr_info("[DEMO_LKM] Module loaded! Function address: %px\n", demo_target_func);
    demo_target_func();
    return 0;
}

static void __exit demo_exit(void) {
    pr_info("[DEMO_LKM] Module unloaded!\n");
}

module_init(demo_init);
module_exit(demo_exit);
```

### `Makefile`
```makefile
obj-m += demo_mod.o

KDIR ?= /lib/modules/$(shell uname -r)/build
PWD  := $(shell pwd)

default:
	$(MAKE) -C $(KDIR) M=$(PWD) modules

clean:
	$(MAKE) -C $(KDIR) M=$(PWD) clean
```

---

## 🔨 3. Building the LKM on Alpine Linux

On Alpine Linux, install build dependencies and compile the kernel module:

```bash
# Install build tools and kernel headers
apk add build-base linux-virt-dev

# Build module
make

# Verify demo_mod.ko was produced
ls -la demo_mod.ko
```

---

## 🎯 4. Live GDB Kernel Debugging Step-by-Step

### Step 4.1: Load Module and Get Text Base Address
Inside Alpine Linux:
```bash
insmod demo_mod.ko

# Find the .text section base address
cat /sys/module/demo_mod/sections/.text
# Example output: 0xffff800000a30000
```

### Step 4.2: Connect GDB from macOS Host
On the macOS host:
```bash
/opt/homebrew/bin/gdb vmlinux \
  -ex "set architecture aarch64" \
  -ex "target remote 127.0.0.1:49930"
```

### Step 4.3: Load Module Symbols in GDB
```gdb
(gdb) add-symbol-file demo_mod.ko 0xffff800000a30000
add symbol table from file "demo_mod.ko" at
        .text_addr = 0xffff800000a30000
(y or n) y
Reading symbols from demo_mod.ko...done.
```

### Step 4.4: Set Breakpoint and Trigger Execution
```gdb
(gdb) b demo_target_func
Breakpoint 1 at 0xffff800000a30010: file demo_mod.c, line 10.

(gdb) continue
Continuing.
```

When `demo_target_func()` is executed inside the VM, GDB hits the breakpoint:
```gdb
Thread 1 hit Breakpoint 1, demo_target_func () at demo_mod.c:10
10          pr_info("[DEMO_LKM] ====> Inside demo_target_func()!\n");
```

---

## 🔍 5. ARM64 Register & Instruction Inspection

### Inspect Registers (`info registers`):
```gdb
(gdb) info registers pc sp cpsr
pc             0xffff800000a30010  0xffff800000a30010 <demo_target_func>
sp             0xffff8000815d3d60  0xffff8000815d3d60
cpsr           0x614000c5          [ SP EL=1 F I BTYPE=0 PAN DIT C Z ]
```

* **`SP EL=1`**: Execution is running in Exception Level 1 (Kernel Mode).
* **`PAN`**: Hardware Privileged Access Never protection is enabled.

### Disassemble Instructions (`disassemble`):
```gdb
(gdb) disassemble
Dump of assembler code for function demo_target_func:
=> 0xffff800000a30010 <+0>:  paciasp
   0xffff800000a30014 <+4>:  stp  x29, x30, [sp, #-16]!
   0xffff800000a30018 <+8>:  mov  x29, sp
   0xffff800000a3001c <+12>: adrp x0, 0xffff800000a30000
   0xffff800000a30020 <+16>: bl   0xffff800080351230 <pr_info>
   0xffff800000a30024 <+20>: ldp  x29, x30, [sp], #16
   0xffff800000a30028 <+24>: autiasp
   0xffff800000a3002c <+28>: ret
End of assembler dump.
```

### Single Instruction Stepping (`stepi`):
```gdb
(gdb) stepi
0xffff800000a30014 in demo_target_func () at demo_mod.c:10
```

---

## 🛡️ Observed ARM64 Security Mechanisms

| Mechanism | Flag / Instruction | Description |
|---|---|---|
| **PAN** | CPSR bit `PAN` (`0x614000c5`) | Privileged Access Never (prevents kernel from accessing user memory unexpectedly) |
| **PAC** | `paciasp` / `autiasp` | Pointer Authentication Code (secures return addresses on stack against ROP) |
| **PXN** | Page Table Attribute | Privileged Execute Never (prevents kernel from executing code in user pages) |
