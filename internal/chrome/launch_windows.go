package chrome

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

// LaunchWithDebugPort starts a second Chrome instance with
// --remote-debugging-port=9222. On Windows, this usually opens a new
// window in the existing Chrome and enables the debug server.
func LaunchWithDebugPort() error {
	chromePath := findChromePath()
	if chromePath == "" {
		return fmt.Errorf("chrome.exe not found")
	}

	cmd := exec.Command(chromePath,
		"--remote-debugging-port=9222",
		"--no-first-run",
		"--no-default-browser-check",
	)
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start: %w", err)
	}

	// Wait for debug port to be ready
	for i := 0; i < 15; i++ {
		if cdpCheckPort("http://localhost:9222") {
			return nil
		}
		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("debug port not ready after 15s")
}

func findChromePath() string {
	candidates := []string{
		filepath.Join(os.Getenv("PROGRAMFILES"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("PROGRAMFILES(X86)"), "Google", "Chrome", "Application", "chrome.exe"),
		filepath.Join(os.Getenv("LOCALAPPDATA"), "Google", "Chrome", "Application", "chrome.exe"),
	}
	for _, p := range candidates {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return ""
}
