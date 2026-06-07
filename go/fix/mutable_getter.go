package fix

import (
	"fmt"
	"regexp"
	"strings"
)

func fixMutableGetter(path string, line int, ext string) (*Result, error) {
	lines, err := readFile(path)
	if err != nil {
		return nil, err
	}
	if line < 1 || line > len(lines) {
		return nil, fmt.Errorf("line %d out of range", line)
	}

	src := lines[line-1]

	switch ext {
	case ".cpp", ".cc", ".cxx", ".h", ".hpp":
		return fixCppMutableGetter(lines, line, src, path)
	case ".java":
		return fixJavaMutableGetter(lines, line, src, path)
	case ".ts", ".tsx":
		return fixTsMutableGetter(lines, line, src, path)
	default:
		return nil, fmt.Errorf("no mutable-getter fix for %s files", ext)
	}
}

// --- C++: add const to return type ---

var cppMutableRefRe = regexp.MustCompile(
	`^(\s*)([\w<>:,\s]+?)(\s*&\s*)(\w+\s*\([^)]*\))`,
)

func fixCppMutableGetter(lines []string, line int, src, path string) (*Result, error) {
	m := cppMutableRefRe.FindStringSubmatch(src)
	if m == nil {
		return nil, fmt.Errorf("cannot parse C++ method return at line %d", line)
	}
	// Insert const before the type
	lines[line-1] = m[1] + "const " + m[2] + m[3] + m[4] + extractSuffix(src, m[0])

	if err := writeFile(path, lines); err != nil {
		return nil, err
	}
	return &Result{
		FilePath: path,
		Diff:     fmt.Sprintf("Changed return type to const reference at line %d", line),
		Warning:  "Callers that mutate through this reference will now fail to compile — fix them.",
	}, nil
}

func extractSuffix(full, matched string) string {
	idx := strings.Index(full, matched)
	if idx < 0 {
		return ""
	}
	return full[idx+len(matched):]
}

// --- Java: wrap return in Collections.unmodifiable ---

var javaReturnRe = regexp.MustCompile(`\breturn\s+(.+?)\s*;`)

func fixJavaMutableGetter(lines []string, line int, src, path string) (*Result, error) {
	// Find the return statement in the method body (search from line onwards)
	for i := line - 1; i < len(lines) && i < line+20; i++ {
		m := javaReturnRe.FindStringSubmatch(lines[i])
		if m != nil {
			expr := m[1]
			// Determine wrapper based on return type in method signature
			wrapper := guessJavaWrapper(lines[line-1])
			lines[i] = strings.Replace(lines[i],
				"return "+expr+";",
				fmt.Sprintf("return %s(%s);", wrapper, expr), 1)

			if err := writeFile(path, lines); err != nil {
				return nil, err
			}
			return &Result{
				FilePath: path,
				Diff:     fmt.Sprintf("Wrapped return value with %s() at line %d", wrapper, i+1),
				Warning:  "Callers that mutate the returned collection will get UnsupportedOperationException at runtime.",
			}, nil
		}
	}
	return nil, fmt.Errorf("cannot find return statement near line %d", line)
}

func guessJavaWrapper(methodSig string) string {
	lower := strings.ToLower(methodSig)
	switch {
	case strings.Contains(lower, "list"):
		return "Collections.unmodifiableList"
	case strings.Contains(lower, "set"):
		return "Collections.unmodifiableSet"
	case strings.Contains(lower, "map"):
		return "Collections.unmodifiableMap"
	default:
		return "Collections.unmodifiableCollection"
	}
}

// --- TypeScript: add readonly to return type ---

var tsReturnTypeRe = regexp.MustCompile(`\):\s*(.+?)\s*\{`)

func fixTsMutableGetter(lines []string, line int, src, path string) (*Result, error) {
	m := tsReturnTypeRe.FindStringSubmatch(src)
	if m != nil {
		oldType := m[1]
		var newType string
		if strings.HasSuffix(oldType, "[]") {
			newType = "readonly " + oldType
		} else if strings.HasPrefix(oldType, "Array<") {
			newType = "Readonly" + oldType
		} else if strings.HasPrefix(oldType, "Map<") {
			newType = "ReadonlyMap" + oldType[3:]
		} else if strings.HasPrefix(oldType, "Set<") {
			newType = "ReadonlySet" + oldType[3:]
		} else {
			newType = "Readonly<" + oldType + ">"
		}
		lines[line-1] = strings.Replace(src, oldType, newType, 1)

		if err := writeFile(path, lines); err != nil {
			return nil, err
		}
		return &Result{
			FilePath: path,
			Diff:     fmt.Sprintf("Changed return type: %s → %s", oldType, newType),
			Warning:  "Callers that mutate the returned value will now get type errors.",
		}, nil
	}
	return nil, fmt.Errorf("cannot parse TypeScript return type at line %d", line)
}
