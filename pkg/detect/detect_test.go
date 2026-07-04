package detect

import (
	"reflect"
	"testing"

	"stack-guard/pkg/github"
)

func TestSelectFiles_ReturnsUniqueSortedRuleInputs(t *testing.T) {
	allPaths := []string{
		"README.md",
		"package.json",
		"build.gradle.kts",
		"pyproject.toml",
		"requirements.txt",
		"package.json",
	}

	selected := SelectFiles(allPaths)
	expected := []string{"build.gradle.kts", "package.json", "pyproject.toml", "requirements.txt"}
	if !reflect.DeepEqual(selected, expected) {
		t.Fatalf("SelectFiles() = %#v, expected %#v", selected, expected)
	}
}

func TestRun_DetectionFixtures(t *testing.T) {
	testCases := []struct {
		name         string
		snapshot     github.RepoSnapshot
		expectedTech []string
	}{
		{
			name: "node react typescript",
			snapshot: github.RepoSnapshot{
				Tree: []github.TreeEntry{
					{Path: "package.json", Type: "blob"},
					{Path: "tsconfig.json", Type: "blob"},
					{Path: "src/app.tsx", Type: "blob"},
					{Path: "src/index.ts", Type: "blob"},
					{Path: "src/util.ts", Type: "blob"},
					{Path: "src/feature.ts", Type: "blob"},
					{Path: "src/worker.ts", Type: "blob"},
				},
				Files: map[string]string{
					"package.json": `{"dependencies":{"react":"18.0.0"},"devDependencies":{"typescript":"5.0.0"}}`,
				},
			},
			expectedTech: []string{"Node", "React", "TypeScript"},
		},
		{
			name: "kotlin gradle spring junit",
			snapshot: github.RepoSnapshot{
				Tree: []github.TreeEntry{
					{Path: "build.gradle.kts", Type: "blob"},
					{Path: "src/main/App.kt", Type: "blob"},
					{Path: "src/test/AppTest.kt", Type: "blob"},
				},
				Files: map[string]string{
					"build.gradle.kts": `implementation("org.springframework.boot:spring-boot-starter") testImplementation("org.junit.jupiter:junit-jupiter")`,
				},
			},
			expectedTech: []string{"Gradle", "JUnit", "Kotlin", "Spring"},
		},
		{
			name: "python ruff pytest",
			snapshot: github.RepoSnapshot{
				Tree: []github.TreeEntry{
					{Path: "pyproject.toml", Type: "blob"},
					{Path: "ruff.toml", Type: "blob"},
					{Path: "app.py", Type: "blob"},
				},
				Files: map[string]string{
					"pyproject.toml": `dependencies = ["pytest", "ruff"]`,
				},
			},
			expectedTech: []string{"Pytest", "Python", "Ruff"},
		},
		{
			name: "readme only",
			snapshot: github.RepoSnapshot{
				Tree:  []github.TreeEntry{{Path: "README.md", Type: "blob"}},
				Files: map[string]string{},
			},
			expectedTech: []string{},
		},
		{
			name: "truncated snapshot still detects",
			snapshot: github.RepoSnapshot{
				Truncated: true,
				Tree: []github.TreeEntry{
					{Path: "go.mod", Type: "blob"},
					{Path: "main.go", Type: "blob"},
				},
				Files: map[string]string{},
			},
			expectedTech: []string{"Go"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := Run(testCase.snapshot)
			actualNames := make([]string, 0, len(actual))
			for _, detected := range actual {
				actualNames = append(actualNames, detected.Name)
			}
			if !reflect.DeepEqual(actualNames, testCase.expectedTech) {
				t.Fatalf("Run() names = %#v, expected %#v", actualNames, testCase.expectedTech)
			}
		})
	}
}

func TestRun_AggregatesConfidenceAndEvidenceDeterministically(t *testing.T) {
	snapshot := github.RepoSnapshot{
		Tree: []github.TreeEntry{
			{Path: "package.json", Type: "blob"},
			{Path: "tsconfig.json", Type: "blob"},
			{Path: "src/app.ts", Type: "blob"},
			{Path: "src/worker.ts", Type: "blob"},
			{Path: "src/feature.ts", Type: "blob"},
			{Path: "src/more.ts", Type: "blob"},
			{Path: "src/last.ts", Type: "blob"},
		},
		Files: map[string]string{
			"package.json": `{"devDependencies":{"typescript":"5.0.0"}}`,
		},
	}

	first := Run(snapshot)
	second := Run(snapshot)
	if !reflect.DeepEqual(first, second) {
		t.Fatalf("Run() should be deterministic\nfirst=%#v\nsecond=%#v", first, second)
	}

	if len(first) != 2 {
		t.Fatalf("expected 2 detections, got %d", len(first))
	}

	for _, detected := range first {
		if detected.Name != "TypeScript" {
			continue
		}
		if detected.Confidence <= 0.95 {
			t.Fatalf("expected aggregated confidence above single-rule confidence, got %.2f", detected.Confidence)
		}
		if len(detected.EvidenceList) < 2 {
			t.Fatalf("expected multiple evidence items for TypeScript, got %d", len(detected.EvidenceList))
		}
		return
	}

	t.Fatal("expected TypeScript detection")
}
