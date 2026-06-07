package fix

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

func fixPublicField(path string, line int, ext string) (*Result, error) {
	lines, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if line < 1 || line > len(lines) {
		return nil, fmt.Errorf("line %d out of range", line)
	}

	src := lines[line-1]

	switch ext {
	case ".java":
		return fixJavaField(lines, line, src, path)
	case ".cpp", ".cc", ".cxx", ".h", ".hpp":
		return fixCppField(lines, line, src, path)
	case ".ts", ".tsx":
		return fixTsField(lines, line, src, path)
	case ".go":
		return fixGoField(lines, line, src, path)
	default:
		return nil, fmt.Errorf("no public-field fix for %s files", ext)
	}
}

// --- Java ---

var javaFieldRe = regexp.MustCompile(
	`^(\s*)public\s+(\w[\w<>,\s\[\]]*?)\s+(\w+)\s*[;=]`,
)

func fixJavaField(lines []string, line int, src, path string) (*Result, error) {
	m := javaFieldRe.FindStringSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("cannot parse Java field at line %d", line)
	}
	indent, typ, name := m[1], m[2], m[3]
	upper := strings.ToUpper(name[:1]) + name[1:]

	// Make private
	lines[line-1] = strings.Replace(src, "public ", "private ", 1)

	// Generate getter + setter after the field
	getter := fmt.Sprintf("%spublic %s get%s() { return this.%s; }", indent, typ, upper, name)
	setter := fmt.Sprintf("%spublic void set%s(%s %s) { this.%s = %s; }", indent, upper, typ, name, name, name)

	insert := []string{"", getter, setter}
	lines = insertLines(lines, line, insert)

	if err := writeFile(path, lines); err != nil {
		return nil, err
	}
	return &Result{
		FilePath: path,
		Diff:     fmt.Sprintf("Encapsulated %s: private + get%s()/set%s()", name, upper, upper),
	}, nil
}

// --- C++ ---

var cppFieldRe = regexp.MustCompile(
	`^(\s*)(\w[\w<>:,\s*&]*?)\s+(\w+)\s*[;=]`,
)

func fixCppField(lines []string, line int, src, path string) (*Result, error) {
	m := cppFieldRe.FindStringSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("cannot parse C++ field at line %d", line)
	}
	indent, typ, name := m[1], m[2], m[3]

	// Rename field to name_
	privateName := name + "_"
	lines[line-1] = fmt.Sprintf("%s%s %s;", indent, typ, privateName)

	// Generate const getter + setter
	getter := fmt.Sprintf("%sconst %s& %s() const { return %s; }", indent, typ, name, privateName)
	setter := fmt.Sprintf("%svoid set_%s(const %s& val) { %s = val; }", indent, name, typ, privateName)

	insert := []string{"", getter, setter}
	lines = insertLines(lines, line, insert)

	if err := writeFile(path, lines); err != nil {
		return nil, err
	}
	return &Result{
		FilePath: path,
		Diff:     fmt.Sprintf("Encapsulated %s → %s with %s()/set_%s()", name, privateName, name, name),
	}, nil
}

// --- TypeScript ---

var tsFieldRe = regexp.MustCompile(
	`^(\s*)(?:public\s+)?(\w+)(\??):\s*(.+?)\s*;`,
)

func fixTsField(lines []string, line int, src, path string) (*Result, error) {
	m := tsFieldRe.FindStringSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("cannot parse TypeScript field at line %d", line)
	}
	indent, name, optional, typ := m[1], m[2], m[3], m[4]

	privateName := "#" + name
	lines[line-1] = fmt.Sprintf("%s%s%s: %s;", indent, privateName, optional, typ)

	getter := fmt.Sprintf("%sget %s(): %s { return this.%s; }", indent, name, typ, privateName)
	setter := fmt.Sprintf("%sset %s(val: %s) { this.%s = val; }", indent, name, typ, privateName)

	insert := []string{"", getter, setter}
	lines = insertLines(lines, line, insert)

	if err := writeFile(path, lines); err != nil {
		return nil, err
	}
	return &Result{
		FilePath: path,
		Diff:     fmt.Sprintf("Encapsulated %s → %s with get/set accessors", name, privateName),
	}, nil
}

// --- Go ---

func fixGoField(lines []string, line int, src, path string) (*Result, error) {
	// Go exported field: leading uppercase identifier
	trimmed := strings.TrimSpace(src)
	parts := strings.Fields(trimmed)
	if len(parts) < 2 || !unicode.IsUpper(rune(parts[0][0])) {
		return nil, fmt.Errorf("cannot parse Go field at line %d", line)
	}
	name := parts[0]
	typ := strings.Join(parts[1:], " ")
	// Remove trailing tags if present
	if idx := strings.Index(typ, "`"); idx >= 0 {
		typ = strings.TrimSpace(typ[:idx])
	}

	lower := strings.ToLower(name[:1]) + name[1:]
	indent := src[:len(src)-len(strings.TrimLeft(src, " \t"))]

	// Make unexported
	lines[line-1] = indent + lower + " " + strings.TrimSpace(src[len(indent)+len(name):])

	// Getter and setter (to be added after the struct closing brace — caller should place manually)
	getter := fmt.Sprintf("func (o *RECEIVER) %s() %s { return o.%s }", name, typ, lower)
	setter := fmt.Sprintf("func (o *RECEIVER) Set%s(v %s) { o.%s = v }", name, typ, lower)

	if err := writeFile(path, lines); err != nil {
		return nil, err
	}
	return &Result{
		FilePath: path,
		Diff:     fmt.Sprintf("Made %s unexported → %s. Add methods:\n  %s\n  %s", name, lower, getter, setter),
		Warning:  "Go getter/setter methods must be added outside the struct. Replace RECEIVER with the type name.",
	}, nil
}

// --- helpers ---

func insertLines(lines []string, after int, insert []string) []string {
	result := make([]string, 0, len(lines)+len(insert))
	result = append(result, lines[:after]...)
	result = append(result, insert...)
	result = append(result, lines[after:]...)
	return result
}
