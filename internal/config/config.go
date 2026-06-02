// Package config manages cookinc's on-disk configuration.
// Source (Windows) and sink (Linux) each have their own config file.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SourceConfig is the Windows-side config.
type SourceConfig struct {
	Sink     SinkRef     `yaml:"sink" json:"sink"`
	Chrome   ChromeRef   `yaml:"chrome" json:"chrome"`
	Peer     PeerRef     `yaml:"peer,omitempty" json:"peer,omitempty"`
	Security SecurityRef `yaml:"security,omitempty" json:"security,omitempty"`
	Allowlist Allowlist  `yaml:"allowlist" json:"allowlist"`
	Watch    WatchConfig `yaml:"watch" json:"watch"`
}

// SinkConfig is the Linux-side config.
type SinkConfig struct {
	Listen   ListenRef   `yaml:"listen" json:"listen"`
	Chrome   ChromeRef   `yaml:"chrome" json:"chrome"`
	Peer     PeerRef     `yaml:"peer,omitempty" json:"peer,omitempty"`
	Security SecurityRef `yaml:"security,omitempty" json:"security,omitempty"`
	MCP      MCPRef      `yaml:"mcp" json:"mcp"`
}

type SinkRef struct {
	URL string `yaml:"url" json:"url"`
}

type ListenRef struct {
	Addr string `yaml:"addr" json:"addr"`
}

type ChromeRef struct {
	DBPath string `yaml:"db_path,omitempty" json:"db_path,omitempty"`
}

type PeerRef struct {
	Hostname string `yaml:"hostname" json:"hostname"`
}

type SecurityRef struct {
	SharedSecret string `yaml:"shared_secret,omitempty" json:"-"`
}

type MCPRef struct {
	Enabled bool   `yaml:"enabled" json:"enabled"`
	Addr    string `yaml:"addr,omitempty" json:"addr,omitempty"`
	DBPath  string `yaml:"db_path,omitempty" json:"db_path,omitempty"`
}

// Allowlist defines which domains are allowed to sync.
type Allowlist struct {
	Domains []string `yaml:"domains" json:"domains"`
}

type WatchConfig struct {
	Interval string `yaml:"interval,omitempty" json:"interval,omitempty"` // e.g. "5s"
}

// DefaultConfigDir returns ~/.config/cookinc/ or %APPDATA%/cookinc/.
func DefaultConfigDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("home dir: %w", err)
	}
	return filepath.Join(home, ".config", "cookinc"), nil
}

// LoadSource reads source.yaml from dir.
func LoadSource(dir string) (*SourceConfig, error) {
	path := filepath.Join(dir, "source.yaml")
	var cfg SourceConfig
	if err := loadYAML(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.Sink.URL == "" {
		return nil, fmt.Errorf("%s: sink.url is required", path)
	}
	if len(cfg.Allowlist.Domains) == 0 {
		return nil, fmt.Errorf("%s: allowlist.domains cannot be empty (sync nothing by default)", path)
	}
	if cfg.Chrome.DBPath == "" {
		cfg.Chrome.DBPath = defaultWinChromeCookiesPath()
	}
	return &cfg, nil
}

// LoadSink reads sink.yaml from dir.
func LoadSink(dir string) (*SinkConfig, error) {
	path := filepath.Join(dir, "sink.yaml")
	var cfg SinkConfig
	if err := loadYAML(path, &cfg); err != nil {
		return nil, err
	}
	if cfg.Listen.Addr == "" {
		return nil, fmt.Errorf("%s: listen.addr is required", path)
	}
	if cfg.Security.SharedSecret == "" && cfg.Peer.Hostname == "" {
		return nil, fmt.Errorf("%s: either security.shared_secret or peer.hostname is required", path)
	}
	if cfg.Chrome.DBPath == "" {
		cfg.Chrome.DBPath = defaultLinuxChromeCookiesPath()
	}
	if cfg.MCP.Addr == "" {
		cfg.MCP.Addr = "127.0.0.1:9898"
	}
	return &cfg, nil
}

func defaultWinChromeCookiesPath() string {
	return filepath.Join(os.Getenv("LOCALAPPDATA"),
		"Google", "Chrome", "User Data", "Default", "Network", "Cookies")
}

func defaultLinuxChromeCookiesPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".config", "google-chrome", "Default", "Cookies")
}

func loadYAML(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("not found: %s (run `cookinc init` to create)", path)
		}
		return fmt.Errorf("open %s: %w", path, err)
	}
	defer f.Close()
	dec := yaml.NewDecoder(f)
	dec.KnownFields(true)
	if err := dec.Decode(out); err != nil {
		return fmt.Errorf("decode %s: %w", path, err)
	}
	return nil
}
