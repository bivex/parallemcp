// Package tools wires Parallels operations into MCP tool handlers. Each tool
// takes a typed input struct (whose JSON schema is inferred by the SDK) and
// returns markdown text content; failures are flagged with IsError.
package tools

import (
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/parallels"
)

// noOut is the unstructured-output sentinel: handlers return text content only,
// with no structured output schema.
type noOut = struct{}

// Tools holds the Parallels client shared by all tool handlers.
type Tools struct {
	cli *parallels.Client
}

func textResult(md string) *mcp.CallToolResult {
	return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: md}}}
}

// errResult flags a human-friendly message as a tool error (IsError=true) so the
// LLM can see and self-correct.
func errResult(msg string) *mcp.CallToolResult {
	r := textResult(msg)
	r.SetError(errors.New(msg))
	return r
}

// fail wraps a Parallels CLI error into a friendly error result, mapping the
// common "VM not found" case to a clear message.
func fail(what string, err error) *mcp.CallToolResult {
	var ee *parallels.ExitError
	if errors.As(err, &ee) && ee.NotFound() {
		return errResult(what + ": VM not found")
	}
	return errResult(what + ": " + err.Error())
}

// Register attaches all Parallels tools to s.
func Register(s *mcp.Server) {
	t := &Tools{cli: parallels.New()}
	t.registerLifecycle(s)
	t.registerSnapshots(s)
	t.registerProvisioning(s)
	t.registerOps(s)
	t.registerSharedFolders(s)
	t.registerFileCopy(s)
	t.registerNetwork(s)
	t.registerSMB(s)
	t.registerExtraTools(s)
}

func ipDisp(s string) string {
	if s == "" || s == "-" {
		return "—"
	}
	return s
}
