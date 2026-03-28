// Package golang implements Go source file checking using go/ast.
package golang

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"

	"github.com/rotmistrk/mcp-lint/go/checks"
	"github.com/rotmistrk/mcp-lint/go/config"
)

// Check analyzes a Go source file and returns all standards violations.
func Check(path string, cfg *config.Config) ([]checks.Violation, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	isTest := strings.HasSuffix(path, "_test.go")

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.AllErrors)
	if err != nil {
		return nil, fmt.Errorf("parse file: %w", err)
	}

	var violations []checks.Violation
	violations = append(violations, checkLineWidth(src, cfg)...)
	if !isTest {
		if cfg.Go.ForbidPanic {
			violations = append(violations, checkPanicCalls(fset, file)...)
		}
		if cfg.Go.ForbidTypeAssertions {
			violations = append(violations, checkTypeAssertions(fset, file)...)
		}
	}
	violations = append(violations, checkFunctions(fset, file, cfg)...)
	return violations, nil
}

func checkLineWidth(src []byte, cfg *config.Config) []checks.Violation {
	var violations []checks.Violation
	for i, line := range strings.Split(string(src), "\n") {
		if len(line) > cfg.MaxLineWidth {
			violations = append(violations, checks.Violation{
				Line:     i + 1,
				Rule:     "line-width",
				Message:  fmt.Sprintf("line is %d chars (max %d)", len(line), cfg.MaxLineWidth),
				Severity: "error",
			})
		}
	}
	return violations
}

func checkPanicCalls(fset *token.FileSet, file *ast.File) []checks.Violation {
	var violations []checks.Violation
	ast.Inspect(file, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		ident, ok := call.Fun.(*ast.Ident)
		if ok && ident.Name == "panic" {
			violations = append(violations, checks.Violation{
				Line:     fset.Position(call.Pos()).Line,
				Rule:     "no-panic",
				Message:  "panic() is forbidden in non-test code",
				Severity: "error",
			})
		}
		return true
	})
	return violations
}

func checkTypeAssertions(
	fset *token.FileSet,
	file *ast.File,
) []checks.Violation {
	var violations []checks.Violation
	ast.Inspect(file, func(n ast.Node) bool {
		ta, ok := n.(*ast.TypeAssertExpr)
		if !ok {
			return true
		}
		if ta.Type == nil { // type switch — allowed
			return true
		}
		violations = append(violations, checks.Violation{
			Line:     fset.Position(ta.Pos()).Line,
			Rule:     "no-type-assert",
			Message:  "type assertion indicates design flaw; use interfaces",
			Severity: "error",
		})
		return true
	})
	return violations
}

func checkFunctions(
	fset *token.FileSet,
	file *ast.File,
	cfg *config.Config,
) []checks.Violation {
	var violations []checks.Violation
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		violations = append(violations, checkOneFunction(fset, fn, cfg)...)
	}
	return violations
}

func checkOneFunction(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	cfg *config.Config,
) []checks.Violation {
	var violations []checks.Violation
	name := fn.Name.Name
	startLine := fset.Position(fn.Body.Lbrace).Line
	endLine := fset.Position(fn.Body.Rbrace).Line
	length := endLine - startLine - 1

	if length > cfg.MaxMethodLength {
		violations = append(violations, checks.Violation{
			Line:     fset.Position(fn.Pos()).Line,
			Rule:     "method-length",
			Message:  fmt.Sprintf("%s is %d lines (max %d)", name, length, cfg.MaxMethodLength),
			Severity: "error",
		})
	}

	maxDepth := maxNestingDepth(fn.Body, 0)
	if maxDepth > cfg.MaxNestingDepth {
		violations = append(violations, checks.Violation{
			Line:     fset.Position(fn.Pos()).Line,
			Rule:     "nesting-depth",
			Message:  fmt.Sprintf("%s has nesting depth %d (max %d)", name, maxDepth, cfg.MaxNestingDepth),
			Severity: "error",
		})
	}

	violations = append(violations, checkParams(fset, fn, cfg)...)
	return violations
}

func maxNestingDepth(node ast.Node, depth int) int {
	max := depth
	ast.Inspect(node, func(n ast.Node) bool {
		if n == node {
			return true
		}
		switch n.(type) {
		case *ast.IfStmt, *ast.ForStmt, *ast.RangeStmt,
			*ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			child := maxNestingDepth(n, depth+1)
			if child > max {
				max = child
			}
			return false
		}
		return true
	})
	return max
}

func checkParams(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	cfg *config.Config,
) []checks.Violation {
	if fn.Type.Params == nil {
		return nil
	}

	var violations []checks.Violation
	params := fn.Type.Params.List

	total := countParams(params)
	if total > cfg.MaxParams {
		violations = append(violations, checks.Violation{
			Line:     fset.Position(fn.Pos()).Line,
			Rule:     "param-count",
			Message:  fmt.Sprintf("%s has %d parameters (max %d)", fn.Name.Name, total, cfg.MaxParams),
			Severity: "error",
		})
	}

	violations = append(
		violations,
		checkConsecutiveSameType(fset, fn, params, cfg)...,
	)
	return violations
}

func countParams(params []*ast.Field) int {
	total := 0
	for _, p := range params {
		if len(p.Names) == 0 {
			total++
		} else {
			total += len(p.Names)
		}
	}
	return total
}

func checkConsecutiveSameType(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	params []*ast.Field,
	cfg *config.Config,
) []checks.Violation {
	var types []string
	for _, p := range params {
		ts := typeString(p.Type)
		count := len(p.Names)
		if count == 0 {
			count = 1
		}
		for range count {
			types = append(types, ts)
		}
	}

	run := 1
	for i := 1; i < len(types); i++ {
		if types[i] == types[i-1] {
			run++
			if run > cfg.MaxConsecutiveSameType {
				return []checks.Violation{{
					Line:     fset.Position(fn.Pos()).Line,
					Rule:     "consecutive-same-type",
					Message:  fmt.Sprintf("%s has %d consecutive %s parameters (max %d)", fn.Name.Name, run, types[i], cfg.MaxConsecutiveSameType),
					Severity: "error",
				}}
			}
		} else {
			run = 1
		}
	}
	return nil
}

func typeString(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.SelectorExpr:
		return typeString(t.X) + "." + t.Sel.Name
	case *ast.StarExpr:
		return "*" + typeString(t.X)
	case *ast.ArrayType:
		return "[]" + typeString(t.Elt)
	case *ast.MapType:
		return "map[" + typeString(t.Key) + "]" + typeString(t.Value)
	case *ast.InterfaceType:
		return "interface{}"
	case *ast.Ellipsis:
		return "..." + typeString(t.Elt)
	default:
		return fmt.Sprintf("%T", expr)
	}
}
