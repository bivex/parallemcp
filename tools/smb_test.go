package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func TestVMSMBValidation(t *testing.T) {
	tools := &Tools{cli: parallels.New()}

	// Missing VM
	res, _, _ := tools.vmSMB(context.Background(), &mcp.CallToolRequest{}, vmSMBInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Missing share_name / folder_path for share action
	res, _, _ = tools.vmSMB(context.Background(), &mcp.CallToolRequest{}, vmSMBInput{
		VM: "vm", Action: "share",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "share_name") {
		t.Errorf("expected share args error: %s", textOf(t, res))
	}

	// Missing remote_ip / share_name for mount action
	res, _, _ = tools.vmSMB(context.Background(), &mcp.CallToolRequest{}, vmSMBInput{
		VM: "vm", Action: "mount",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "remote_ip") {
		t.Errorf("expected mount args error: %s", textOf(t, res))
	}
}
