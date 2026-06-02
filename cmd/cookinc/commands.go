package main

import (
	"fmt"

	"github.com/spf13/cobra"
)

func initCmd() *cobra.Command {
	var sinkURL string
	var secret string
	var allowlist []string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create source.yaml configuration",
		Long:  `Creates ~/.config/cookinc/source.yaml with your sink URL, shared secret, and allowlist.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Printf("cookinc init — not yet implemented\n")
			fmt.Printf("  sink-url:  %s\n", sinkURL)
			fmt.Printf("  allowlist: %v\n", allowlist)
			return nil
		},
	}

	cmd.Flags().StringVar(&sinkURL, "sink-url", "", "Sink URL (e.g. http://vps:9876)")
	cmd.Flags().StringVar(&secret, "secret", "", "Shared secret (min 32 chars)")
	cmd.Flags().StringSliceVar(&allowlist, "allowlist", []string{}, "Comma-separated domains to sync")
	cmd.MarkFlagRequired("sink-url")
	cmd.MarkFlagRequired("secret")
	cmd.MarkFlagRequired("allowlist")

	return cmd
}

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the cookie watcher and sync loop",
		Long:  `Watches Chrome's Cookies SQLite for changes, decrypts via DPAPI, filters by allowlist, and pushes encrypted payload to the sink.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cookinc start — not yet implemented")
			fmt.Println("Will: watch → decrypt → filter → encrypt → POST")
			return nil
		},
	}
}

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync health and last push info",
		RunE: func(cmd *cobra.Command, args []string) error {
			fmt.Println("cookinc status — not yet implemented")
			return nil
		},
	}
}
