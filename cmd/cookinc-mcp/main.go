// cookinc-mcp is the Linux-side MCP server.
// It receives cookies from the Windows daemon, writes them to the local
// Chrome store, and exposes them via the Model Context Protocol so
// AI agents (Hermes, Claude Code, Cursor) can query sessions directly.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("cookinc-mcp — Linux MCP server (not yet implemented)")
	fmt.Println("Provides: get_cookies(domain), list_domains(), sync_status()")
	return nil
}
