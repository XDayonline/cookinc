//go:build !windows

package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the bridge server for Chrome extension",
		Long:  `Start is only supported on Windows (Chrome extension + DPAPI).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return fmt.Errorf("cookinc start is only supported on Windows")
		},
	}
}
