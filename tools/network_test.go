package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func TestVMNetworkValidation(t *testing.T) {
	tools := &Tools{cli: parallels.New()}

	// Missing VM
	res, _, _ := tools.vmNetwork(context.Background(), &mcp.CallToolRequest{}, vmNetworkInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "vm") {
		t.Errorf("expected vm error: %s", textOf(t, res))
	}

	// Invalid action
	res, _, _ = tools.vmNetwork(context.Background(), &mcp.CallToolRequest{}, vmNetworkInput{
		VM: "vm", Action: "unknown",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "action") {
		t.Errorf("expected action error: %s", textOf(t, res))
	}
}
