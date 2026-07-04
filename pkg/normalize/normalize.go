// Package normalize maps technology names and aliases to canonical names.
package normalize

import "strings"

var aliases = map[string]string{
	".net":           ".NET",
	"c#":             "C#",
	"csharp":         "C#",
	"docker":         "Docker",
	"eslint":         "ESLint",
	"github actions": "GitHub Actions",
	"github-actions": "GitHub Actions",
	"go":             "Go",
	"gradle":         "Gradle",
	"java":           "Java",
	"javascript":     "JavaScript",
	"jest":           "Jest",
	"junit":          "JUnit",
	"js":             "JavaScript",
	"kotlin":         "Kotlin",
	"maven":          "Maven",
	"node":           "Node",
	"node.js":        "Node",
	"nodejs":         "Node",
	"pytest":         "Pytest",
	"python":         "Python",
	"react":          "React",
	"ruff":           "Ruff",
	"ruby":           "Ruby",
	"rust":           "Rust",
	"spring":         "Spring",
	"spring boot":    "Spring",
	"spring-boot":    "Spring",
	"testcontainers": "Testcontainers",
	"ts":             "TypeScript",
	"typescript":     "TypeScript",
	"vite":           "Vite",
}

// Canonical normalizes a technology name into a canonical form.
func Canonical(name string) string {
	normalized := normalizeKey(name)
	if normalized == "" {
		return ""
	}

	if canonical, ok := aliases[normalized]; ok {
		return canonical
	}

	return titleWords(normalized)
}

func normalizeKey(name string) string {
	trimmed := strings.TrimSpace(name)
	if trimmed == "" {
		return ""
	}

	return strings.ToLower(trimmed)
}

// titleWords capitalizes the first letter of each word in a string, where words are separated by '-', '_', '/', or whitespace.
// e.g. "spring-boot_tools/v2" becomes "Spring Boot Tools V2".
// This is used to normalize technology names that are not in the aliases map.
func titleWords(value string) string {
	separators := func(r rune) bool {
		switch r {
		case '-', '_', '/', ' ':
			return true
		default:
			return false
		}
	}

	parts := strings.FieldsFunc(value, separators)
	if len(parts) == 0 {
		return ""
	}

	for index := range parts {
		part := parts[index]
		if part == "" {
			continue
		}
		parts[index] = strings.ToUpper(part[:1]) + part[1:]
	}

	return strings.Join(parts, " ")
}
