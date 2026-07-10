package main

import (
	"os"

	"github.com/spf13/cobra"
)

func main() {
	root := &cobra.Command{
		Use:   "cookinc",
		Short: "Chrome cookie sync from Windows to your Linux agent",
		Long: `cookinc syncs your Chrome sessions from Windows to a Linux sink
where AI agents (Hermes, Claude Code, Cursor) can read them.

Cross-platform, encrypted (AES-256-GCM), allowlist-only.`,
	}

	root.AddCommand(initCmd())
	root.AddCommand(startCmd())
	root.AddCommand(statusCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}
