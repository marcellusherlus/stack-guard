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

	allowlist, set, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist returned error: %v", err)
	}

	if len(allowlist.Languages) != 2 {
		t.Fatalf("expected 2 unique languages, got %d", len(allowlist.Languages))
	}

	if !set.Contains("TypeScript") {
		t.Fatal("expected TypeScript to be in canonical set")
	}

	if !set.Contains("python") {
		t.Fatal("expected python alias lookup to resolve")
	}

	if set.Contains("unknown-tech") {
		t.Fatal("did not expect unknown-tech to be in canonical set")
	}
}

func TestLoadAllowlist_MissingKeysBecomeEmptySlices(t *testing.T) {
	tempDir := t.TempDir()
	allowlistPath := filepath.Join(tempDir, "allowlist.json")

	if err := os.WriteFile(allowlistPath, []byte(`{"languages": ["Go"]}`), 0o600); err != nil {
		t.Fatalf("write allowlist: %v", err)
	}

	allowlist, _, err := LoadAllowlist(allowlistPath)
	if err != nil {
		t.Fatalf("LoadAllowlist returned error: %v", err)
	}

	if allowlist.Frameworks == nil || allowlist.Tools == nil {
		t.Fatal("expected missing keys to result in non-nil empty slices")
	}
}

func TestLoadAllowlist_Errors(t *testing.T) {
	t.Run("missing file", func(t *testing.T) {
		_, _, err := LoadAllowlist(filepath.Join(t.TempDir(), "missing.json"))
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

		_, _, err := LoadAllowlist(allowlistPath)
		if err == nil {
			t.Fatal("expected json parse error")
		}
	})
}
