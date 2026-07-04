package main

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"stack-guard/pkg/types"
)

func stubFetchRepositoryReport(t *testing.T, stub func() (types.Report, error)) {
	t.Helper()
	original := fetchRepositoryReport
	fetchRepositoryReport = func(_ context.Context, _, _ string, _ types.Allowlist, _ bool) (types.Report, error) {
		return stub()
	}
	t.Cleanup(func() {
		fetchRepositoryReport = original
	})
}

func TestParseConfig_Errors(t *testing.T) {
	testCases := []struct {
		name string
		args []string
	}{
		{
			name: "missing repository",
			args: []string{"--allowlist", "allowlist.json"},
		},
		{
			name: "missing allowlist",
			args: []string{"org/repo"},
		},
		{
			name: "invalid repository",
			args: []string{"--allowlist", "allowlist.json", "org"},
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			_, err := parseConfig(testCase.args, os.Stderr)
			if err == nil {
				t.Fatal("expected parseConfig to fail")
			}
		})
	}
}

func TestRun_ValidInput(t *testing.T) {
	stubFetchRepositoryReport(t, func() (types.Report, error) {
		return types.Report{
			Repository: "org/repo",
			Verdict:    types.VerdictCompliant,
			Detected: []types.ClassifiedTech{
				{DetectedTech: types.DetectedTech{Name: "Go", Category: types.CategoryLanguage, Confidence: 0.95}, Allowed: true},
			},
		}, nil
	})

	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")
	content := `{"languages": ["Go"]}`
	if err := os.WriteFile(allowlistPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := run([]string{"--allowlist", allowlistPath, "org/repo"}, &stdout, &stderr)
	if exitCode != 0 {
		t.Fatalf("expected exit code 0, got %d (stderr=%q)", exitCode, stderr.String())
	}

	if !strings.Contains(stdout.String(), "Repository: org/repo") {
		t.Fatalf("unexpected stdout: %q", stdout.String())
	}
	if !strings.Contains(stdout.String(), "Verdict:") {
		t.Fatalf("expected rendered verdict, got %q", stdout.String())
	}
}

func TestRun_InvalidInputExitCode(t *testing.T) {
	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := run([]string{"--allowlist", "missing.json", "org/repo"}, &stdout, &stderr)
	if exitCode != 2 {
		t.Fatalf("expected exit code 2, got %d", exitCode)
	}

	if !strings.Contains(stderr.String(), "input error") {
		t.Fatalf("expected input error output, got %q", stderr.String())
	}
}

func TestRun_RuntimeErrorExitCode(t *testing.T) {
	stubFetchRepositoryReport(t, func() (types.Report, error) {
		return types.Report{}, errors.New("fetch failed")
	})

	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")
	content := `{"languages": ["Go"]}`
	if err := os.WriteFile(allowlistPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := run([]string{"--allowlist", allowlistPath, "org/repo"}, &stdout, &stderr)
	if exitCode != 3 {
		t.Fatalf("expected exit code 3, got %d", exitCode)
	}
	if !strings.Contains(stderr.String(), "runtime error: fetch failed") {
		t.Fatalf("unexpected stderr: %q", stderr.String())
	}
}

func TestRun_NonCompliantExitCode(t *testing.T) {
	stubFetchRepositoryReport(t, func() (types.Report, error) {
		return types.Report{Repository: "org/repo", Verdict: types.VerdictNonCompliant}, nil
	})

	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")
	content := `{"languages": ["Go"]}`
	if err := os.WriteFile(allowlistPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	var stdout strings.Builder
	var stderr strings.Builder

	exitCode := run([]string{"--allowlist", allowlistPath, "org/repo"}, &stdout, &stderr)
	if exitCode != 1 {
		t.Fatalf("expected exit code 1, got %d", exitCode)
	}
}
