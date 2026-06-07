// Package fix provides automated fixes for lint violations.
package fix

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// Result holds the fix output.
type Result struct {
	FilePath string `json:"file_path"`
	Diff     string `json:"diff"`
	Warning  string `json:"warning,omitempty"`
}

// Fix applies an automated fix for a given rule at the specified location.
func Fix(path string, line int, rule string) (*Result, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch rule {
	case "no-public-fields", "no-public-members", "no-pub-fields":
		return fixPublicField(path, line, ext)
	case "no-mutable-getters":
		return fixMutableGetter(path, line, ext)
	default:
		return nil, fmt.Errorf("no auto-fix available for rule %q", rule)
	}
}

func readFile(path string) ([]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return strings.Split(string(data), "\n"), nil
}

func writeFile(path string, lines []string) error {
	return os.WriteFile(path, []byte(strings.Join(lines, "\n")), 0644)
}
