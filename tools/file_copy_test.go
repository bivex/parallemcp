package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func TestVMFileCopyValidation(t *testing.T) {
	tools := &Tools{cli: parallels.New()}

	// Missing args
	res, _, _ := tools.vmFileCopy(context.Background(), &mcp.CallToolRequest{}, vmFileCopyInput{})
	if !res.IsError || !strings.Contains(textOf(t, res), "required") {
		t.Errorf("expected required fields error: %s", textOf(t, res))
	}

	// Invalid direction
	res, _, _ = tools.vmFileCopy(context.Background(), &mcp.CallToolRequest{}, vmFileCopyInput{
		VM: "vm", HostPath: "/tmp/a", GuestPath: "C:\\a", Direction: "invalid",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "direction") {
		t.Errorf("expected direction error: %s", textOf(t, res))
	}
}
