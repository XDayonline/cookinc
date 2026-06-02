package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"

	"github.com/XDayonline/cookinc/internal/config"
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
			dir, err := config.DefaultConfigDir()
			if err != nil {
				return fmt.Errorf("config dir: %w", err)
			}
			if err := os.MkdirAll(dir, 0700); err != nil {
				return fmt.Errorf("mkdir %s: %w", dir, err)
			}

			hostname, _ := os.Hostname()

			cfg := config.SourceConfig{
				Sink: config.SinkRef{URL: sinkURL},
				Chrome: config.ChromeRef{
					DBPath: filepath.Join(os.Getenv("LOCALAPPDATA"),
						"Google", "Chrome", "User Data", "Default", "Network", "Cookies"),
				},
				Peer: config.PeerRef{
					Hostname: hostname,
				},
				Security: config.SecurityRef{
					SharedSecret: secret,
				},
				Allowlist: config.Allowlist{
					Domains: allowlist,
				},
				Watch: config.WatchConfig{
					Interval: "5s",
				},
			}

			path := filepath.Join(dir, "source.yaml")
			f, err := os.Create(path)
			if err != nil {
				return fmt.Errorf("create %s: %w", path, err)
			}
			defer f.Close()

			enc := yaml.NewEncoder(f)
			enc.SetIndent(2)
			if err := enc.Encode(&cfg); err != nil {
				return fmt.Errorf("encode yaml: %w", err)
			}
			enc.Close()

			fmt.Printf("Config written to %s\n", path)
			fmt.Printf("  sink URL:    %s\n", sinkURL)
			fmt.Printf("  hostname:    %s\n", hostname)
			fmt.Printf("  allowlist:   %v\n", allowlist)
			fmt.Printf("  interval:    5s\n")
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

func statusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show sync health and last push info",
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.DefaultConfigDir()
			if err != nil {
				return fmt.Errorf("config dir: %w", err)
			}

			cfg, err := config.LoadSource(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			fmt.Println("cookinc status")
			fmt.Println("==============")
			fmt.Printf("Config dir:  %s\n", dir)
			fmt.Printf("Sink URL:    %s\n", cfg.Sink.URL)
			fmt.Printf("Hostname:    %s\n", cfg.Peer.Hostname)
			fmt.Printf("DB path:     %s\n", cfg.Chrome.DBPath)
			fmt.Printf("Allowlist:   %v\n", cfg.Allowlist.Domains)
			fmt.Printf("Interval:    %s\n", cfg.Watch.Interval)

			if _, err := os.Stat(cfg.Chrome.DBPath); os.IsNotExist(err) {
				fmt.Println("DB status:   NOT FOUND (Chrome cookies DB missing)")
			} else {
				fmt.Println("DB status:   OK")
			}

			fmt.Println("\nRun 'cookinc start' to begin syncing.")
			return nil
		},
	}
}
