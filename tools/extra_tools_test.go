package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func TestExtraToolsValidation(t *testing.T) {
	tools := &Tools{cli: parallels.New()}

	// Screenshot missing VM
	res, _, _ := tools.vmScreenshot(context.Background(), &mcp.CallToolRequest{}, vmScreenshotInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Compact missing VM
	res, _, _ = tools.vmCompact(context.Background(), &mcp.CallToolRequest{}, vmCompactInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// CDROM missing action
	res, _, _ = tools.vmCDROM(context.Background(), &mcp.CallToolRequest{}, vmCDROMInput{VM: "vm"})
	if !res.IsError || !strings.Contains(textOf(t, res), "action") {
		t.Errorf("expected action error: %s", textOf(t, res))
	}

	// Snapshot delete missing ID
	res, _, _ = tools.vmSnapshotDelete(context.Background(), &mcp.CallToolRequest{}, vmSnapshotDeleteInput{VM: "vm"})
	if !res.IsError || !strings.Contains(textOf(t, res), "id") {
		t.Errorf("expected id error: %s", textOf(t, res))
	}

	// Disk manage missing size
	res, _, _ = tools.vmDiskManage(context.Background(), &mcp.CallToolRequest{}, vmDiskManageInput{VM: "vm", Action: "add"})
	if !res.IsError || !strings.Contains(textOf(t, res), "size") {
		t.Errorf("expected size error: %s", textOf(t, res))
	}

	// Bundle missing path / vm
	res, _, _ = tools.vmBundle(context.Background(), &mcp.CallToolRequest{}, vmBundleInput{Action: "register"})
	if !res.IsError || !strings.Contains(textOf(t, res), "path") {
		t.Errorf("expected path error: %s", textOf(t, res))
	}

	// Configure debugger missing VM
	res, _, _ = tools.vmConfigureDebugger(context.Background(), &mcp.CallToolRequest{}, vmConfigureDebuggerInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Guest debugger missing VM
	res, _, _ = tools.vmGuestDebugger(context.Background(), &mcp.CallToolRequest{}, vmGuestDebuggerInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Configure kernel debug missing VM
	res, _, _ = tools.vmConfigureKernelDebug(context.Background(), &mcp.CallToolRequest{}, vmConfigureKernelDebugInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Debug registers missing target
	res, _, _ = tools.vmDebugRegisters(context.Background(), &mcp.CallToolRequest{}, vmDebugRegistersInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "target") {
		t.Errorf("expected target error: %s", textOf(t, res))
	}

	// Debug step missing target
	res, _, _ = tools.vmDebugStep(context.Background(), &mcp.CallToolRequest{}, vmDebugStepInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "target") {
		t.Errorf("expected target error: %s", textOf(t, res))
	}

	// Debug disassemble missing target
	res, _, _ = tools.vmDebugDisassemble(context.Background(), &mcp.CallToolRequest{}, vmDebugDisassembleInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "target") {
		t.Errorf("expected target error: %s", textOf(t, res))
	}

	// Debug serial missing socket_path
	res, _, _ = tools.vmDebugSerial(context.Background(), &mcp.CallToolRequest{}, vmDebugSerialInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "socket_path") {
		t.Errorf("expected socket_path error: %s", textOf(t, res))
	}
}


