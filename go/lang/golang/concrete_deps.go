package golang

import (
	"fmt"
	"go/ast"
	"go/token"

	"github.com/rotmistrk/mcp-lint/go/checks"
)

// checkConcreteDeps flags function parameters whose type is a concrete struct
// rather than an interface. Only checks parameters — not return types or locals.
func checkConcreteDeps(
	fset *token.FileSet,
	file *ast.File,
) []checks.Violation {
	ifaces, structs := collectTypes(file)
	var violations []checks.Violation

	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Type.Params == nil {
			continue
		}
		violations = append(
			violations,
			checkFuncParams(fset, fn, ifaces, structs)...,
		)
	}
	return violations
}

func checkFuncParams(
	fset *token.FileSet,
	fn *ast.FuncDecl,
	ifaces map[string]bool,
	structs map[string]bool,
) []checks.Violation {
	var violations []checks.Violation
	for _, param := range fn.Type.Params.List {
		if isConcreteType(param.Type, ifaces, structs) {
			typeName := typeString(param.Type)
			for _, name := range param.Names {
				violations = append(violations, checks.Violation{
					Line:     fset.Position(name.Pos()).Line,
					Rule:     "no-concrete-deps",
					Message: fmt.Sprintf(
						"%s: parameter %s has concrete type %s; use an interface",
						fn.Name.Name, name.Name, typeName,
					),
					Severity: "error",
				})
			}
			if len(param.Names) == 0 {
				violations = append(violations, checks.Violation{
					Line:     fset.Position(param.Pos()).Line,
					Rule:     "no-concrete-deps",
					Message:  fmt.Sprintf("%s: parameter has concrete type %s; use an interface", fn.Name.Name, typeName),
					Severity: "error",
				})
			}
		}
	}
	return violations
}

func isConcreteType(
	expr ast.Expr,
	ifaces map[string]bool,
	structs map[string]bool,
) bool {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return isConcreteType(t.X, ifaces, structs)
	case *ast.Ident:
		return isConcreteName(t.Name, ifaces, structs)
	case *ast.SelectorExpr:
		name := typeString(expr)
		return isConcretePkgType(name, ifaces)
	default:
		return false
	}
}

func isConcreteName(
	name string,
	ifaces map[string]bool,
	structs map[string]bool,
) bool {
	if isPrimitive(name) || isKnownInterface(name) || ifaces[name] {
		return false
	}
	return structs[name]
}

func isConcretePkgType(name string, ifaces map[string]bool) bool {
	if isStdlibInterface(name) || ifaces[name] {
		return false
	}
	if isStdlibConcrete(name) {
		return true
	}
	return false
}

func collectTypes(file *ast.File) (ifaces map[string]bool, structs map[string]bool) {
	ifaces = make(map[string]bool)
	structs = make(map[string]bool)
	for _, decl := range file.Decls {
		genDecl, ok := decl.(*ast.GenDecl)
		if !ok || genDecl.Tok != token.TYPE {
			continue
		}
		for _, spec := range genDecl.Specs {
			ts, ok := spec.(*ast.TypeSpec)
			if !ok {
				continue
			}
			switch ts.Type.(type) {
			case *ast.InterfaceType:
				ifaces[ts.Name.Name] = true
			case *ast.StructType:
				structs[ts.Name.Name] = true
			}
		}
	}
	return ifaces, structs
}

func isPrimitive(name string) bool {
	switch name {
	case "bool", "byte", "rune",
		"int", "int8", "int16", "int32", "int64",
		"uint", "uint8", "uint16", "uint32", "uint64", "uintptr",
		"float32", "float64", "complex64", "complex128",
		"string", "error", "any":
		return true
	}
	return false
}

func isKnownInterface(name string) bool {
	switch name {
	case "error", "any", "Reader", "Writer", "Closer",
		"ReadWriter", "ReadCloser", "WriteCloser",
		"ReadWriteCloser", "Stringer", "Handler",
		"Context":
		return true
	}
	return false
}

func isStdlibInterface(name string) bool {
	switch name {
	case "io.Reader", "io.Writer", "io.Closer",
		"io.ReadWriter", "io.ReadCloser", "io.WriteCloser",
		"io.ReadWriteCloser", "io.ReaderFrom", "io.WriterTo",
		"io.ByteReader", "io.ByteWriter", "io.RuneReader",
		"fmt.Stringer", "fmt.GoStringer",
		"sort.Interface",
		"context.Context",
		"http.Handler", "http.ResponseWriter",
		"net.Conn", "net.Listener",
		"encoding.BinaryMarshaler", "encoding.BinaryUnmarshaler",
		"encoding.TextMarshaler", "encoding.TextUnmarshaler",
		"json.Marshaler", "json.Unmarshaler",
		"sql.Scanner", "driver.Value":
		return true
	}
	return false
}

func isStdlibConcrete(name string) bool {
	switch name {
	case "os.File", "bytes.Buffer", "strings.Builder",
		"sync.Mutex", "sync.RWMutex", "sync.WaitGroup",
		"http.Client", "http.Server", "http.Request",
		"sql.DB", "sql.Tx", "sql.Rows":
		return true
	}
	return false
}
