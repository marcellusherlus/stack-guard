package input

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLoadAllowlist_ValidAndCanonicalized(t *testing.T) {
	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")

	content := `{
		"languages": ["python", "ts", "Python"],
		"frameworks": ["react"],
		"tools": ["ruff", "Docker"]
	}`

	if err := os.WriteFile(allowlistPath, []byte(content), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	allowlist, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist returned error: %v", err)
	}

	if len(allowlist.Languages) != 2 {
		t.Fatalf("expected 2 unique languages, got %d", len(allowlist.Languages))
	}

	if allowlist.Languages[0] != "Python" || allowlist.Languages[1] != "TypeScript" {
		t.Fatalf("expected canonicalized languages [Python TypeScript], got %v", allowlist.Languages)
	}

	if len(allowlist.Frameworks) != 1 || allowlist.Frameworks[0] != "React" {
		t.Fatalf("expected canonicalized frameworks [React], got %v", allowlist.Frameworks)
	}

	if len(allowlist.Tools) != 2 || allowlist.Tools[0] != "Ruff" || allowlist.Tools[1] != "Docker" {
		t.Fatalf("expected canonicalized tools [Ruff Docker], got %v", allowlist.Tools)
	}
}

func TestLoadAllowlist_MissingKeysBecomeEmptySlices(t *testing.T) {
	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")

	if err := os.WriteFile(allowlistPath, []byte(`{"languages": ["Go"]}`), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	allowlist, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist returned error: %v", err)
	}

	if allowlist.Frameworks == nil || allowlist.Tools == nil {
		t.Fatal("expected missing keys to result in non-nil empty slices")
	}
}

func TestLoadAllowlist_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, err := LoadAllowlist(filepath.Join(t.TempDir(), "missing.json"))
		if err == nil {
			t.Fatal("expected error for missing file")
		}
	})

	t.Run("invalid json", func(t *testing.T) {
		tempDir := t.TempDir()
		allowlistPath := filepath.Join(tempDir, "allowlist.json")
		if err := os.WriteFile(allowlistPath, []byte(`{"languages":`), 0o600); err != nil {
			t.Fatalf("write allowlist: %v", err)
		}

		_, err := LoadAllowlist(allowlistPath)
		if err == nil {
			t.Fatal("expected json parse error")
		}
	})
}
