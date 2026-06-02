//go:build !windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the cookie watcher and sync loop",
		Long:  `Start is only supported on Windows. This command is a stub for cross-compilation.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cookinc start is only supported on Windows (DPAPI required)")
		},
	}
}
