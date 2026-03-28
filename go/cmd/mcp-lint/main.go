// Command mcp-lint is an MCP server that checks source files against
// coding standards. Dispatches to per-language checkers.
package main

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"

	"github.com/rotmistrk/mcp-lint/go/checks"
	"github.com/rotmistrk/mcp-lint/go/config"
	"github.com/rotmistrk/mcp-lint/go/lang/dispatch"
)

func main() {
	s := server.NewMCPServer("mcp-lint", "0.1.0")
	registerCheckFile(s)

	stdio := server.NewStdioServer(s)
	if err := stdio.Listen(context.Background(), os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "mcp-lint: %v\n", err)
		os.Exit(1)
	}
}

func registerCheckFile(s *server.MCPServer) {
	s.AddTool(
		mcp.NewTool(
			"check_file",
			mcp.WithDescription(
				"Check a source file against coding standards. "+
					"Supports: Go, TypeScript, Rust, C++, Java. "+
					"Language detected from file extension.",
			),
			mcp.WithString(
				"path",
				mcp.Required(),
				mcp.Description("Path to the source file to check"),
			),
		),
		handleCheckFile,
	)
}

func handleCheckFile(
	ctx context.Context,
	req mcp.CallToolRequest,
) (*mcp.CallToolResult, error) {
	path, ok := requireString(req, "path")
	if !ok {
		return errResult("missing required argument: path"), nil
	}

	dir, _ := os.Getwd()
	cfg, _ := config.Load(dir)

	violations, err := dispatch.CheckFile(path, cfg)
	if err != nil {
		return errResult("check failed: %v", err), nil
	}

	return formatViolations(path, violations), nil
}

func formatViolations(
	path string,
	violations []checks.Violation,
) *mcp.CallToolResult {
	if len(violations) == 0 {
		return textResult(fmt.Sprintf("%s: OK (no violations)", path))
	}

	var sb strings.Builder
	fmt.Fprintf(&sb, "%s: %d violation(s)\n\n", path, len(violations))
	for _, v := range violations {
		fmt.Fprintf(&sb, "  line %d: [%s] %s\n", v.Line, v.Rule, v.Message)
	}
	return textResult(sb.String())
}

func requireString(req mcp.CallToolRequest, name string) (string, bool) {
	args := req.GetArguments()
	val, ok := args[name]
	if !ok {
		return "", false
	}
	s, ok := val.(string)
	return s, ok
}

func textResult(text string) *mcp.CallToolResult {
	return mcp.NewToolResultText(text)
}

func errResult(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			mcp.NewTextContent(fmt.Sprintf(format, args...)),
		},
		IsError: true,
	}
}
