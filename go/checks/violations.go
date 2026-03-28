// Package checks defines the shared violation contract for all language checkers.
package checks

import "encoding/json"

// Violation represents a single standards violation found in a source file.
type Violation struct {
	Line     int    `json:"line"`
	Rule     string `json:"rule"`
	Message  string `json:"message"`
	Severity string `json:"severity"` // "error" or "warning"
}

// Result is the output of a checker for a single file.
type Result struct {
	File       string      `json:"file"`
	Violations []Violation `json:"violations"`
}

// MarshalResults serializes results to JSON.
func MarshalResults(results []Result) ([]byte, error) {
	return json.MarshalIndent(results, "", "  ")
}

// UnmarshalViolations parses JSON output from a checker binary.
func UnmarshalViolations(data []byte) ([]Violation, error) {
	var violations []Violation
	err := json.Unmarshal(data, &violations)
	return violations, err
}
