// cookinc is the Windows-side daemon CLI.
// It watches Chrome's cookie store, decrypts via DPAPI, filters by allowlist,
// and pushes encrypted payloads to the configured sink.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func run() error {
	fmt.Println("cookinc — Windows daemon (not yet implemented)")
	fmt.Println("See cmd/cookinc/cmd/ for the cobra subcommand tree")
	return nil
}
