// Command mcp-lint-go checks Go source files against coding standards.
// Standalone binary with JSON output, also usable as a library.
package main

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/rotmistrk/mcp-lint/go/config"
	"github.com/rotmistrk/mcp-lint/go/lang/golang"
)

func main() {
	if len(os.Args) < 3 || os.Args[1] != "check" {
		fmt.Fprintf(os.Stderr, "Usage: mcp-lint-go check <file.go>\n")
		os.Exit(2)
	}

	path := os.Args[2]
	cfg := loadConfig()

	violations, err := golang.Check(path, cfg)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	data, _ := json.MarshalIndent(violations, "", "  ")
	fmt.Println(string(data))

	if len(violations) > 0 {
		os.Exit(1)
	}
}

func loadConfig() *config.Config {
	dir, _ := os.Getwd()
	cfg, err := config.Load(dir)
	if err != nil {
		return config.Defaults()
	}
	return cfg
}
