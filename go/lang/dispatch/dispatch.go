// Package dispatch routes file checks to the appropriate language checker.
package dispatch

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/rotmistrk/mcp-lint/go/checks"
	"github.com/rotmistrk/mcp-lint/go/config"
	"github.com/rotmistrk/mcp-lint/go/lang/golang"
)

// CheckFile dispatches a file to the appropriate checker.
func CheckFile(path string, cfg *config.Config) ([]checks.Violation, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".go":
		return golang.Check(path, cfg)
	case ".ts", ".tsx":
		return runExternal("mcp-lint-ts", path, cfg)
	case ".rs":
		return runExternal("mcp-lint-rs", path, cfg)
	case ".cpp", ".cc", ".cxx", ".h", ".hpp":
		return runExternal("mcp-lint-cpp", path, cfg)
	case ".java":
		return runExternal("mcp-lint-java", path, cfg)
	default:
		return nil, fmt.Errorf("unsupported file type: %s", ext)
	}
}

// runExternal calls an external checker binary and parses its JSON output.
func runExternal(
	binary string,
	path string,
	cfg *config.Config,
) ([]checks.Violation, error) {
	cfgJSON, _ := json.Marshal(cfg)
	cmd := exec.Command(binary, "check", path, "--config-json", string(cfgJSON))
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Non-zero exit with output means violations found — parse it
		if len(out) > 0 {
			return checks.UnmarshalViolations(out)
		}
		return nil, fmt.Errorf("%s not found or failed: %w", binary, err)
	}
	if len(out) == 0 {
		return nil, nil
	}
	return checks.UnmarshalViolations(out)
}
