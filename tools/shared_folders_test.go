package tools

import (
	"context"
	"strings"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

func TestVMSharedFoldersValidation(t *testing.T) {
	tools := &Tools{cli: parallels.New()}

	// Missing VM
	res, _, _ := tools.vmSharedFolders(context.Background(), &mcp.CallToolRequest{}, vmSharedFoldersInput{})
	if !res.IsError {
		t.Fatal("expected IsError when VM is missing")
	}

	// Missing action args
	res, _, _ = tools.vmSharedFolders(context.Background(), &mcp.CallToolRequest{}, vmSharedFoldersInput{
		VM: "vm", Action: "add",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "name") {
		t.Errorf("expected missing name error: %s", textOf(t, res))
	}

	res, _, _ = tools.vmSharedFolders(context.Background(), &mcp.CallToolRequest{}, vmSharedFoldersInput{
		VM: "vm", Action: "remove",
	})
	if !res.IsError || !strings.Contains(textOf(t, res), "name") {
		t.Errorf("expected missing name error: %s", textOf(t, res))
	}
}
