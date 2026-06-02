//go:build windows

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/XDayonline/cookinc/internal/chrome"
	"github.com/XDayonline/cookinc/internal/config"
	"github.com/XDayonline/cookinc/internal/crypto"
	"github.com/XDayonline/cookinc/internal/protocol"
)

func startCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the cookie watcher and sync loop",
		Long: `Watches Chrome's Cookies SQLite for changes, decrypts via DPAPI, filters by allowlist, and pushes encrypted payload to the sink.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, err := config.DefaultConfigDir()
			if err != nil {
				return fmt.Errorf("config dir: %w", err)
			}

			cfg, err := config.LoadSource(dir)
			if err != nil {
				return fmt.Errorf("load config: %w", err)
			}

			log.Printf("cookinc start — loading config from %s", dir)
			log.Printf("  sink:      %s", cfg.Sink.URL)
			log.Printf("  hostname:  %s", cfg.Peer.Hostname)
			log.Printf("  db:        %s", cfg.Chrome.DBPath)
			log.Printf("  allowlist: %v", cfg.Allowlist.Domains)
			log.Printf("  interval:  %s", cfg.Watch.Interval)

			interval, err := time.ParseDuration(cfg.Watch.Interval)
			if err != nil {
				if cfg.Watch.Interval == "" {
					interval = 5 * time.Second
				} else {
					return fmt.Errorf("invalid watch.interval %q: %w", cfg.Watch.Interval, err)
				}
			}

			localStatePath := deriveLocalStatePath(cfg.Chrome.DBPath)

			reader, err := chrome.NewWindowsReader(cfg.Chrome.DBPath, localStatePath)
			if err != nil {
				return fmt.Errorf("chrome reader: %w", err)
			}

			encKey := crypto.DeriveKeyFromSecret(cfg.Security.SharedSecret)
			client := &http.Client{Timeout: 30 * time.Second}

			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			log.Println("cookinc start — running (ctrl+c to stop)")

			ticker := time.NewTicker(interval)
			defer ticker.Stop()

			for {
				select {
				case <-sigCh:
					log.Println("cookinc stop — shutting down")
					return nil

				case <-ticker.C:
					if err := syncOnce(reader, cfg, encKey, client); err != nil {
						log.Printf("sync: %v", err)
					}
				}
			}
		},
	}
}

func syncOnce(reader *chrome.WindowsReader, cfg *config.SourceConfig, key []byte, client *http.Client) error {
	hostname := cfg.Peer.Hostname
	if hostname == "" {
		hostname, _ = os.Hostname()
	}

	cookies, err := reader.ReadCookies(cfg.Allowlist.Domains)
	if err != nil {
		return fmt.Errorf("read: %w", err)
	}

	if len(cookies) == 0 {
		log.Println("sync: no matching cookies found")
		return nil
	}

	env := protocol.NewEnvelope(hostname, cookies)
	plaintext, err := json.Marshal(env)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}

	sealed, err := crypto.Encrypt(key, plaintext)
	if err != nil {
		return fmt.Errorf("encrypt: %w", err)
	}

	resp, err := client.Post(cfg.Sink.URL+"/sync", "application/octet-stream", bytes.NewReader(sealed))
	if err != nil {
		return fmt.Errorf("post: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("sink returned %s", resp.Status)
	}

	log.Printf("sync: pushed %d cookies to %s", len(cookies), cfg.Sink.URL)
	return nil
}

// deriveLocalStatePath computes the Local State path from the cookies DB path.
//
// Cookies.sqlite is at:
//
//	.../Chrome/User Data/Default/Network/Cookies
//
// Local State is at:
//
//	.../Chrome/User Data/Local State
func deriveLocalStatePath(dbPath string) string {
	// Go up: Network -> Default -> User Data
	return filepath.Join(filepath.Dir(filepath.Dir(filepath.Dir(dbPath))), "Local State")
}
