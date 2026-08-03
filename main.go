// Command parallemcp is a Model Context Protocol server that exposes Parallels
// Desktop VM management (prlctl / prlsrvctl) as tools for LLM agents such as
// Claude Code and Claude Desktop. It communicates over stdin/stdout (JSON-RPC).
package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"parallemcp/tools"
)

const version = "0.1.0"

func main() {
	// The MCP transport is stdin/stdout, so every diagnostic MUST go to stderr;
	// any byte on stdout corrupts the JSON-RPC stream.
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	server := mcp.NewServer(&mcp.Implementation{Name: "parallemcp", Version: version}, nil)
	tools.Register(server)

	log.Printf("parallemcp %s: serving Parallels tools over stdio", version)
	if err := server.Run(ctx, &mcp.StdioTransport{}); err != nil &&
		err != context.Canceled {
		log.Fatalf("parallemcp: server error: %v", err)
	}
}
