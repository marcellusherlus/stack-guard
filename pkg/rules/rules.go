package rules

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"stack-guard/pkg/types"
)

const (
	manifestConfidence   = 0.95
	dependencyConfidence = 0.90
	configConfidence     = 0.80
)

// MatchContext contains the repository inputs rules inspect.
type MatchContext struct {
	Paths          []string
	ExtensionCount map[string]int
	FileContents   map[string]string
}

// Rule describes one deterministic signal for a technology.
type Rule struct {
	ID             string
	Tech           string
	Category       types.Category
	NeedsContentOf []string
	Match          func(matchContext MatchContext) *types.Evidence
}

// All returns the full detection rule table.
func All() []Rule {
	return []Rule{
		manifestRule("node-package-json", "Node", types.CategoryTool, "package.json", "package.json", "package.json present"),
		manifestRule("typescript-tsconfig", "TypeScript", types.CategoryLanguage, "tsconfig.json", "tsconfig.json", "tsconfig.json present"),
		manifestRule("python-pyproject", "Python", types.CategoryLanguage, "pyproject.toml", "pyproject.toml", "pyproject.toml present"),
		manifestRule("python-requirements", "Python", types.CategoryLanguage, "requirements.txt", "requirements.txt", "requirements.txt present"),
		manifestRule("gradle-build-kts", "Gradle", types.CategoryTool, "build.gradle.kts", "build.gradle.kts", "build.gradle.kts present"),
		manifestRule("gradle-build", "Gradle", types.CategoryTool, "build.gradle", "build.gradle", "build.gradle present"),
		manifestRule("kotlin-gradle-kts", "Kotlin", types.CategoryLanguage, "build.gradle.kts", "build.gradle.kts", "build.gradle.kts present"),
		manifestRule("maven-pom", "Maven", types.CategoryTool, "pom.xml", "pom.xml", "pom.xml present"),
		manifestRule("java-pom", "Java", types.CategoryLanguage, "pom.xml", "pom.xml", "pom.xml present"),
		manifestRule("go-mod", "Go", types.CategoryLanguage, "go.mod", "go.mod", "go.mod present"),
		manifestRule("cargo-toml", "Rust", types.CategoryLanguage, "Cargo.toml", "Cargo.toml", "Cargo.toml present"),
		manifestRule("gemfile", "Ruby", types.CategoryLanguage, "Gemfile", "Gemfile", "Gemfile present"),
		manifestRule("dockerfile", "Docker", types.CategoryTool, "Dockerfile", "Dockerfile", "Dockerfile present"),
		manifestRule("ruff-config", "Ruff", types.CategoryTool, "ruff.toml", "ruff.toml", "ruff.toml present"),
		manifestRule("ruff-dot-config", "Ruff", types.CategoryTool, ".ruff.toml", ".ruff.toml", ".ruff.toml present"),
		pathPrefixRule("github-actions-workflows", "GitHub Actions", types.CategoryTool, ".github/workflows/", ".github/workflows/* present"),
		pathPrefixRule("eslint-config", "ESLint", types.CategoryTool, ".eslintrc", ".eslintrc* present"),
		pathPrefixRule("jest-config", "Jest", types.CategoryTool, "jest.config.", "jest.config.* present"),
		pathPrefixRule("vite-config", "Vite", types.CategoryTool, "vite.config.", "vite.config.* present"),
		packageJSONDependencyRule("react-package-json", "React", types.CategoryFramework, "react"),
		packageJSONDependencyRule("typescript-package-json", "TypeScript", types.CategoryLanguage, "typescript"),
		containsInAnyFileRule("spring-build-files", "Spring", types.CategoryFramework, []string{"build.gradle", "build.gradle.kts", "pom.xml"}, []string{"org.springframework", "spring-boot-starter"}, "spring dependency found"),
		containsInAnyFileRule("junit-build-files", "JUnit", types.CategoryFramework, []string{"build.gradle", "build.gradle.kts", "pom.xml"}, []string{"junit", "junit-jupiter"}, "junit dependency found"),
		containsInAnyFileRule("testcontainers-build-files", "Testcontainers", types.CategoryTool, []string{"build.gradle", "build.gradle.kts", "pom.xml"}, []string{"testcontainers"}, "testcontainers dependency found"),
		containsInAnyFileRule("pytest-pyproject", "Pytest", types.CategoryTool, []string{"pyproject.toml", "requirements.txt"}, []string{"pytest"}, "pytest dependency found"),
		containsInAnyFileRule("ruff-pyproject", "Ruff", types.CategoryTool, []string{"pyproject.toml", "requirements.txt"}, []string{"ruff"}, "ruff dependency found"),
		extensionRule("kotlin-extension", "Kotlin", types.CategoryLanguage, ".kt"),
		extensionRule("python-extension", "Python", types.CategoryLanguage, ".py"),
		extensionRule("typescript-extension", "TypeScript", types.CategoryLanguage, ".ts", ".tsx"),
		extensionRule("javascript-extension", "JavaScript", types.CategoryLanguage, ".js"),
		extensionRule("java-extension", "Java", types.CategoryLanguage, ".java"),
		extensionRule("go-extension", "Go", types.CategoryLanguage, ".go"),
		extensionRule("rust-extension", "Rust", types.CategoryLanguage, ".rs"),
		extensionRule("ruby-extension", "Ruby", types.CategoryLanguage, ".rb"),
		extensionRule("csharp-extension", "C#", types.CategoryLanguage, ".cs"),
	}
}

func manifestRule(id, tech string, category types.Category, targetPath, source, reason string) Rule {
	return Rule{
		ID:       id,
		Tech:     tech,
		Category: category,
		Match: func(matchContext MatchContext) *types.Evidence {
			if !hasExactPath(matchContext.Paths, targetPath) {
				return nil
			}
			return &types.Evidence{Source: source, Reason: reason, Confidence: manifestConfidence}
		},
	}
}

func pathPrefixRule(id, tech string, category types.Category, prefix, reason string) Rule {
	return Rule{
		ID:       id,
		Tech:     tech,
		Category: category,
		Match: func(matchContext MatchContext) *types.Evidence {
			for _, candidatePath := range matchContext.Paths {
				base := filepath.Base(candidatePath)
				if strings.HasPrefix(candidatePath, prefix) || strings.HasPrefix(base, prefix) {
					return &types.Evidence{Source: candidatePath, Reason: reason, Confidence: configConfidence}
				}
			}
			return nil
		},
	}
}

func packageJSONDependencyRule(id, tech string, category types.Category, dependency string) Rule {
	return Rule{
		ID:             id,
		Tech:           tech,
		Category:       category,
		NeedsContentOf: []string{"package.json"},
		Match: func(matchContext MatchContext) *types.Evidence {
			content, ok := matchContext.FileContents["package.json"]
			if !ok {
				return nil
			}
			if !packageJSONHasDependency(content, dependency) {
				return nil
			}
			return &types.Evidence{Source: "package.json", Reason: dependency + " dependency found", Confidence: dependencyConfidence}
		},
	}
}

func containsInAnyFileRule(id, tech string, category types.Category, files, substrings []string, reason string) Rule {
	return Rule{
		ID:             id,
		Tech:           tech,
		Category:       category,
		NeedsContentOf: append([]string(nil), files...),
		Match: func(matchContext MatchContext) *types.Evidence {
			for _, candidateFile := range files {
				content, ok := matchContext.FileContents[candidateFile]
				if !ok {
					continue
				}
				lowerContent := strings.ToLower(content)
				for _, needle := range substrings {
					if strings.Contains(lowerContent, strings.ToLower(needle)) {
						return &types.Evidence{Source: candidateFile, Reason: reason, Confidence: dependencyConfidence}
					}
				}
			}
			return nil
		},
	}
}

func extensionRule(id, tech string, category types.Category, extensions ...string) Rule {
	return Rule{
		ID:       id,
		Tech:     tech,
		Category: category,
		Match: func(matchContext MatchContext) *types.Evidence {
			count := 0
			for _, extension := range extensions {
				count += matchContext.ExtensionCount[extension]
			}
			if count == 0 {
				return nil
			}
			confidence := 0.40
			if count >= 5 {
				confidence = 0.70
			}
			label := extensionSummary(count, extensions)
			return &types.Evidence{Source: label, Reason: label + " found", Confidence: confidence}
		},
	}
}

func hasExactPath(paths []string, wanted string) bool {
	return slices.Contains(paths, wanted)
}

func packageJSONHasDependency(content, dependency string) bool {
	var document struct {
		Dependencies    map[string]string `json:"dependencies"`
		DevDependencies map[string]string `json:"devDependencies"`
	}
	if err := json.Unmarshal([]byte(content), &document); err != nil {
		return false
	}
	if _, exists := document.Dependencies[dependency]; exists {
		return true
	}
	_, exists := document.DevDependencies[dependency]
	return exists
}

func extensionSummary(count int, extensions []string) string {
	if len(extensions) == 1 {
		return fmtCount(count, extensions[0])
	}
	return fmtCount(count, strings.Join(extensions, "/"))
}

func fmtCount(count int, label string) string {
	return strings.Join([]string{strconv.Itoa(count), label, "files"}, " ")
}
