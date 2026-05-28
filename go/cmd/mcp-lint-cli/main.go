// Command mcp-lint-cli is a unified command-line linter with getopt_long-style flags.
// It dispatches to per-language checkers and enforces coding standards including
// the "no public/exported fields" rule.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/rotmistrk/mcp-lint/go/checks"
	"github.com/rotmistrk/mcp-lint/go/config"
	"github.com/rotmistrk/mcp-lint/go/lang/dispatch"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	opts, files, err := parseFlags(args)
	if err != nil {
		return 2
	}
	if opts.help {
		return 0
	}
	if opts.version {
		fmt.Println("mcp-lint-cli 0.1.0")
		return 0
	}
	if len(files) == 0 {
		fmt.Fprintf(os.Stderr, "error: no input files\n")
		return 2
	}
	return checkFiles(files, opts)
}

type options struct {
	configPath string
	format     string
	help       bool
	version    bool
}

func parseFlags(args []string) (options, []string, error) {
	fs := flag.NewFlagSet("mcp-lint-cli", flag.ContinueOnError)
	fs.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: mcp-lint-cli [OPTIONS] FILE [FILE...]\n\n")
		fmt.Fprintf(os.Stderr, "Check source files against coding standards.\n")
		fmt.Fprintf(os.Stderr, "Supports: Go, TypeScript, Rust, C++, Java.\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		fs.PrintDefaults()
	}

	var opts options
	fs.StringVar(&opts.configPath, "config", "", "path to .mcp-lint.yaml config file")
	fs.StringVar(&opts.configPath, "c", "", "path to .mcp-lint.yaml config file (shorthand)")
	fs.StringVar(&opts.format, "format", "text", "output format: text, json")
	fs.StringVar(&opts.format, "f", "text", "output format (shorthand)")
	fs.BoolVar(&opts.help, "help", false, "show help message")
	fs.BoolVar(&opts.help, "h", false, "show help (shorthand)")
	fs.BoolVar(&opts.version, "version", false, "show version")
	fs.BoolVar(&opts.version, "V", false, "show version (shorthand)")

	if err := fs.Parse(args); err != nil {
		return opts, nil, err
	}
	return opts, fs.Args(), nil
}

func checkFiles(files []string, opts options) int {
	cfg := loadCfg(opts.configPath)
	hasViolations := false

	for _, path := range files {
		violations, err := dispatch.CheckFile(path, cfg)
		if err != nil {
			fmt.Fprintf(os.Stderr, "error: %s: %v\n", path, err)
			return 1
		}
		if len(violations) > 0 {
			hasViolations = true
		}
		printViolations(path, violations, opts.format)
	}

	if hasViolations {
		return 1
	}
	return 0
}

func loadCfg(configPath string) *config.Config {
	if configPath != "" {
		cfg := config.Defaults()
		data, err := os.ReadFile(configPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot read config %s: %v\n", configPath, err)
			return cfg
		}
		if parseErr := json.Unmarshal(data, cfg); parseErr != nil {
			fmt.Fprintf(os.Stderr, "warning: cannot parse config %s: %v\n", configPath, parseErr)
		}
		return cfg
	}
	dir, dirErr := os.Getwd()
	if dirErr != nil {
		return config.Defaults()
	}
	cfg, err := config.Load(dir)
	if err != nil {
		return config.Defaults()
	}
	return cfg
}

func printViolations(
	path string,
	violations []checks.Violation,
	format string,
) {
	switch strings.ToLower(format) {
	case "json":
		printJSON(violations)
	default:
		printText(path, violations)
	}
}

func printJSON(violations []checks.Violation) {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	if err := enc.Encode(violations); err != nil {
		fmt.Fprintf(os.Stderr, "error: encoding json: %v\n", err)
	}
}

func printText(path string, violations []checks.Violation) {
	if len(violations) == 0 {
		fmt.Printf("%s: OK\n", path)
		return
	}
	for _, v := range violations {
		fmt.Printf("%s:%d: [%s] %s\n", path, v.Line, v.Rule, v.Message)
	}
}
