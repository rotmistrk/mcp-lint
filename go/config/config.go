// Package config loads lint configuration from defaults.yaml and .mcp-lint.yaml.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds all lint thresholds and language-specific settings.
type Config struct {
	MaxMethodLength        int `yaml:"max_method_length"`
	MaxNestingDepth        int `yaml:"max_nesting_depth"`
	MaxLineWidth           int `yaml:"max_line_width"`
	MaxParams              int `yaml:"max_params"`
	MaxConsecutiveSameType int `yaml:"max_consecutive_same_type"`
	MaxCodeLinesPerFile    int `yaml:"max_code_lines_per_file"`

	Go         GoConfig         `yaml:"go"`
	Rust       RustConfig       `yaml:"rust"`
	TypeScript TypeScriptConfig `yaml:"typescript"`
	Cpp        CppConfig        `yaml:"cpp"`
	Java       JavaConfig       `yaml:"java"`
}

// GoConfig holds Go-specific settings.
type GoConfig struct {
	ForbidPanic           bool `yaml:"forbid_panic"`
	ForbidTypeAssertions  bool `yaml:"forbid_type_assertions"`
	ForbidExportedFields  bool `yaml:"forbid_exported_fields"`
	ForbidSwallowedErrors bool `yaml:"forbid_swallowed_errors"`
	MaxTypesPerFile       int  `yaml:"max_types_per_file"`
}

// RustConfig holds Rust-specific settings.
type RustConfig struct {
	ForbidUnwrap bool `yaml:"forbid_unwrap"`
	ForbidExpect bool `yaml:"forbid_expect"`
	ForbidPanic  bool `yaml:"forbid_panic"`
}

// TypeScriptConfig holds TypeScript-specific settings.
type TypeScriptConfig struct {
	ForbidAny             bool `yaml:"forbid_any"`
	ForbidClassComponents bool `yaml:"forbid_class_components"`
	ForbidWaitForTimeout  bool `yaml:"forbid_wait_for_timeout"`
	ForbidEmptyCatch      bool `yaml:"forbid_empty_catch"`
}

// CppConfig holds C++-specific settings.
type CppConfig struct {
	ForbidRawNew    bool `yaml:"forbid_raw_new"`
	ForbidCCasts    bool `yaml:"forbid_c_casts"`
	ForbidEmptyCatch bool `yaml:"forbid_empty_catch"`
}

// JavaConfig holds Java-specific settings.
type JavaConfig struct {
	ForbidRawTypes bool `yaml:"forbid_raw_types"`
}

// Defaults returns the default configuration.
func Defaults() *Config {
	return &Config{
		MaxMethodLength:        40,
		MaxNestingDepth:        3,
		MaxLineWidth:           120,
		MaxParams:              7,
		MaxConsecutiveSameType: 2,
		MaxCodeLinesPerFile:    240,
		Go:         GoConfig{ForbidPanic: true, ForbidTypeAssertions: true, ForbidExportedFields: true, ForbidSwallowedErrors: true, MaxTypesPerFile: 1},
		Rust:       RustConfig{ForbidUnwrap: true, ForbidExpect: true, ForbidPanic: true},
		TypeScript: TypeScriptConfig{ForbidAny: true, ForbidClassComponents: true, ForbidWaitForTimeout: true, ForbidEmptyCatch: true},
		Cpp:        CppConfig{ForbidRawNew: true, ForbidCCasts: true, ForbidEmptyCatch: true},
		Java:       JavaConfig{ForbidRawTypes: true},
	}
}

// Load reads config from the project directory, falling back to defaults.
// Searches: .mcp-lint.yaml in dir, then ~/.mcp-lint.yaml, then built-in defaults.
func Load(dir string) (*Config, error) {
	cfg := Defaults()

	// User-level config
	home, err := os.UserHomeDir()
	if err == nil {
		_ = mergeFile(cfg, filepath.Join(home, ".mcp-lint.yaml"))
	}

	// Project-level config
	if dir != "" {
		_ = mergeFile(cfg, filepath.Join(dir, ".mcp-lint.yaml"))
	}

	return cfg, nil
}

func mergeFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return fmt.Errorf("parse %s: %w", path, err)
	}
	return nil
}
