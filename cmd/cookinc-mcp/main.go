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
	"github.com/XDayonline/cookinc/internal/mcp"
	"github.com/XDayonline/cookinc/internal/transport"
)

func main() {
	root := &cobra.Command{
		Use:   "cookinc-mcp",
		Short: "Linux cookie sink + MCP server for AI agents",
		Long: `cookinc-mcp receives Chrome cookies from the Windows cookinc daemon,
stores them in a local sidecar, and exposes them via MCP for AI agents
(Hermes, Claude Code, Cursor).`,
	}

	root.AddCommand(initCmd())
	root.AddCommand(startCmd())
	root.AddCommand(statusCmd())
	root.AddCommand(mcpCmd())

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func initCmd() *cobra.Command {
	var listenAddr string
	var secret string
	var mcpAddr string

	cmd := &cobra.Command{
		Use:   "init",
		Short: "Create sink.yaml configuration",
		Long:  `Creates ~/.config/cookinc/sink.yaml with the sink URL, shared secret, and MCP address.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir, err := config.DefaultConfigDir()
			if err != nil {
				return err
			}
			if err := os.MkdirAll(cfgDir, 0700); err != nil {
				return fmt.Errorf("mkdir config: %w", err)
			}

			cfg := fmt.Sprintf(`# cookinc sink config (Linux)
listen:
  addr: "%s"
security:
  shared_secret: "%s"
mcp:
  enabled: true
  addr: "%s"
`, listenAddr, secret, mcpAddr)

			path := cfgDir + "/sink.yaml"
			if err := os.WriteFile(path, []byte(cfg), 0600); err != nil {
				return fmt.Errorf("write %s: %w", path, err)
			}

			fmt.Printf("Created %s\n", path)
			fmt.Printf("  Listen:    %s\n", listenAddr)
			fmt.Printf("  MCP:       %s\n", mcpAddr)
			fmt.Println()
			fmt.Println("Run 'cookinc-mcp start' to start the server.")
			return nil
		},
	}

	cmd.Flags().StringVar(&listenAddr, "listen", "127.0.0.1:9876", "Listen address for sync endpoint")
	cmd.Flags().StringVar(&secret, "secret", "", "Shared secret (min 32 chars)")
	cmd.Flags().StringVar(&mcpAddr, "mcp-addr", "127.0.0.1:9898", "MCP HTTP server address")
	cmd.MarkFlagRequired("secret")

	return cmd
}

func startCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "start",
		Short: "Start the sync server + MCP server",
		Long:  `Starts the HTTP sync endpoint and optional MCP server.`,
		RunE: func(cmd *cobra.Command, args []string) error {
			// Load config
			cfgDir := configPath
			if cfgDir == "" {
				var err error
				cfgDir, err = config.DefaultConfigDir()
				if err != nil {
					return err
				}
			}

			cfg, err := config.LoadSink(cfgDir)
			if err != nil {
				return fmt.Errorf("load config: %w\nRun 'cookinc-mcp init' first", err)
			}

			// Open sidecar store
			store, err := chrome.NewSidecar(cfg.MCP.DBPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			// Start sink server
			sinkServer := transport.NewSinkServer(store, cfg.Security.SharedSecret, cfg.Listen.Addr)

			// Start MCP if enabled
			if cfg.MCP.Enabled {
				go startMCPServer(store, cfg)
			}

			log.Printf("cookinc-mcp starting on %s", cfg.Listen.Addr)
			return sinkServer.ListenAndServe()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Config directory (default ~/.config/cookinc/)")
	return cmd
}

func startMCPServer(store *chrome.Sidecar, cfg *config.SinkConfig) {
	handler := &mcp.ToolHandler{
		GetCookies: func(domain string) (any, error) {
			cookies, err := store.GetCookies(domain)
			if err != nil {
				return nil, err
			}
			if len(cookies) == 0 {
				return map[string]any{
					"domain":  domain,
					"cookies": []any{},
					"message": "No cookies found for this domain",
				}, nil
			}
			return map[string]any{
				"domain":  domain,
				"count":   len(cookies),
				"cookies": cookies,
			}, nil
		},
		ListDomains: func() (any, error) {
			domains, err := store.ListDomains()
			if err != nil {
				return nil, err
			}
			if domains == nil {
				domains = []string{}
			}
			return map[string]any{
				"count":   len(domains),
				"domains": domains,
			}, nil
		},
		SyncStatus: func() (any, error) {
			count, err := store.Count()
			if err != nil {
				return nil, err
			}
			lastUpdated, _ := store.LastUpdated()
			return map[string]any{
				"cookie_count": count,
				"last_updated": lastUpdated.Format("2006-01-02 15:04:05 MST"),
				"status":       "running",
			}, nil
		},
	}

	// Start MCP in HTTP mode too (for Hermes/curl testing)
	httpServer := mcp.NewHTTPServer(handler, cfg.MCP.Addr)
	log.Printf("cookinc-mcp: MCP HTTP server on %s", cfg.MCP.Addr)
	if err := httpServer.ListenAndServe(); err != nil {
		log.Fatalf("MCP HTTP server: %v", err)
	}
}

func statusCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show sink server status",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir := configPath
			if cfgDir == "" {
				var err error
				cfgDir, err = config.DefaultConfigDir()
				if err != nil {
					return err
				}
			}

			cfg, err := config.LoadSink(cfgDir)
			if err != nil {
				return fmt.Errorf("not configured: %w", err)
			}

			store, err := chrome.NewSidecar(cfg.MCP.DBPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			count, _ := store.Count()
			lastUpdated, _ := store.LastUpdated()
			domains, _ := store.ListDomains()

			fmt.Println("cookinc-mcp status:")
			fmt.Printf("  Sync listener: %s\n", cfg.Listen.Addr)
			fmt.Printf("  MCP server:    %s (enabled: %v)\n", cfg.MCP.Addr, cfg.MCP.Enabled)
			fmt.Printf("  Cookie count:  %d\n", count)
			fmt.Printf("  Last updated:  %s\n", lastUpdated.Format("2006-01-02 15:04:05"))
			fmt.Printf("  Domains:       %d\n", len(domains))
			if len(domains) > 0 {
				for _, d := range domains {
					fmt.Printf("    - %s\n", d)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Config directory (default ~/.config/cookinc/)")
	return cmd
}

func mcpCmd() *cobra.Command {
	var configPath string

	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Run MCP server in stdio mode",
		Long:  `Runs the MCP server over stdio (for Claude Desktop, Cursor, Hermes integration).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			cfgDir := configPath
			if cfgDir == "" {
				var err error
				cfgDir, err = config.DefaultConfigDir()
				if err != nil {
					return err
				}
			}

			cfg, err := config.LoadSink(cfgDir)
			if err != nil {
				return fmt.Errorf("load config: %w\nRun 'cookinc-mcp init' first", err)
			}

			store, err := chrome.NewSidecar(cfg.MCP.DBPath)
			if err != nil {
				return fmt.Errorf("open store: %w", err)
			}
			defer store.Close()

			handler := &mcp.ToolHandler{
				GetCookies: func(domain string) (any, error) {
					cookies, err := store.GetCookies(domain)
					if err != nil {
						return nil, err
					}
					return map[string]any{
						"domain":  domain,
						"count":   len(cookies),
						"cookies": cookies,
					}, nil
				},
				ListDomains: func() (any, error) {
					domains, err := store.ListDomains()
					if err != nil {
						return nil, err
					}
					return map[string]any{
						"count":   len(domains),
						"domains": domains,
					}, nil
				},
				SyncStatus: func() (any, error) {
					count, err := store.Count()
					if err != nil {
						return nil, err
					}
					lastUpdated, _ := store.LastUpdated()
					return map[string]any{
						"cookie_count": count,
						"last_updated": lastUpdated.Format("2006-01-02 15:04:05 MST"),
						"status":       "running",
					}, nil
				},
			}

			// Trap SIGINT/SIGTERM for clean shutdown
			sigCh := make(chan os.Signal, 1)
			signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
			go func() {
				<-sigCh
				os.Exit(0)
			}()

			server := mcp.New(handler)
			return server.Run()
		},
	}

	cmd.Flags().StringVar(&configPath, "config", "", "Config directory (default ~/.config/cookinc/)")
	return cmd
}
