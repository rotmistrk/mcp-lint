package golang

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rotmistrk/mcp-lint/go/checks"
	"github.com/rotmistrk/mcp-lint/go/config"
)

func writeTempGo(t *testing.T, name, content string) string {
	t.Helper()
	dir := t.TempDir()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func findRule(violations []checks.Violation, rule string) bool {
	for _, v := range violations {
		if v.Rule == rule {
			return true
		}
	}
	return false
}

var cfg = config.Defaults()

func TestCheck_Clean(t *testing.T) {
	path := writeTempGo(t, "clean.go", "package foo\n\nfunc Short() { _ = 1 }\n")
	v, err := Check(path, cfg)
	if err != nil {
		t.Fatal(err)
	}
	if len(v) > 0 {
		t.Errorf("expected no violations, got %v", v)
	}
}

func TestCheck_MethodLength(t *testing.T) {
	var sb strings.Builder
	sb.WriteString("package foo\n\nfunc TooLong() {\n")
	for i := range 45 {
		sb.WriteString("\t_ = " + string(rune('a'+i%26)) + "\n")
	}
	sb.WriteString("}\n")

	v, err := Check(writeTempGo(t, "long.go", sb.String()), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "method-length") {
		t.Error("expected method-length violation")
	}
}

func TestCheck_NestingDepth(t *testing.T) {
	v, err := Check(writeTempGo(t, "nested.go", `package foo

func Deep() {
	if true {
		if true {
			if true {
				if true { _ = 1 }
			}
		}
	}
}
`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "nesting-depth") {
		t.Error("expected nesting-depth violation")
	}
}

func TestCheck_Panic(t *testing.T) {
	v, err := Check(writeTempGo(t, "p.go", "package foo\n\nfunc Bad() { panic(\"x\") }\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "no-panic") {
		t.Error("expected no-panic violation")
	}
}

func TestCheck_PanicAllowedInTests(t *testing.T) {
	v, err := Check(writeTempGo(t, "p_test.go", "package foo\n\nfunc TestX() { panic(\"ok\") }\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if findRule(v, "no-panic") {
		t.Error("panic should be allowed in test files")
	}
}

func TestCheck_TypeAssertion(t *testing.T) {
	v, err := Check(writeTempGo(t, "ta.go", "package foo\n\nfunc Bad(x interface{}) { _ = x.(string) }\n"), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "no-type-assert") {
		t.Error("expected no-type-assert violation")
	}
}

func TestCheck_TypeSwitchAllowed(t *testing.T) {
	v, err := Check(writeTempGo(t, "ts.go", `package foo

func Ok(x interface{}) {
	switch x.(type) {
	case string:
		_ = 1
	}
}
`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if findRule(v, "no-type-assert") {
		t.Error("type switch should be allowed")
	}
}

func TestCheck_ParamCount(t *testing.T) {
	v, err := Check(writeTempGo(t, "pc.go", `package foo

func TooMany(a, b, c, d, e, f, g, h int) { _ = a+b+c+d+e+f+g+h }
`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "param-count") {
		t.Error("expected param-count violation")
	}
}

func TestCheck_ConsecutiveSameType(t *testing.T) {
	v, err := Check(writeTempGo(t, "cst.go", `package foo

func Bad(a string, b string, c string) { _ = a+b+c }
`), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "consecutive-same-type") {
		t.Error("expected consecutive-same-type violation")
	}
}

func TestCheck_LineWidth(t *testing.T) {
	long := "package foo\n\nvar x = \"" + strings.Repeat("a", 120) + "\"\n"
	v, err := Check(writeTempGo(t, "wide.go", long), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !findRule(v, "line-width") {
		t.Error("expected line-width violation")
	}
}
