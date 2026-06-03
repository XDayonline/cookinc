//go:build windows

package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"github.com/XDayonline/cookinc/internal/chrome"
	"github.com/XDayonline/cookinc/internal/config"
)

func startCmd() *cobra.Command {
	var listenAddr string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the bridge server for Chrome extension",
		Long: `Starts a local HTTP server that receives cookies from the Cookinc Chrome extension.
The extension decrypts cookies internally (handles App-Bound Encryption)
and pushes them to this server, which encrypts and forwards to the sink.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.DefaultConfigDir()
			if err != nil {
				return fmt.Errorf("config dir: %w", err)
			}

			cfg, err := config.LoadSource(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			hostname := cfg.Peer.Hostname
			if hostname == "" {
				hostname, _ = os.Hostname()
			}

			log.Printf("cookinc start — bridge mode")
			log.Printf("  sink:      %s", cfg.Sink.URL)
			log.Printf("  hostname:  %s", hostname)
			log.Printf("  allowlist: %v", cfg.Allowlist.Domains)

			server := chrome.NewBridgeServer(
				listenAddr,
				cfg.Allowlist.Domains,
				cfg.Security.SharedSecret,
				cfg.Sink.URL,
				hostname,
			)

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				log.Println("cookinc stop — shutting down")
				os.Exit(0)
			}()

			return server.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:19999", "Address for Chrome extension callback")

	return cmd
}
