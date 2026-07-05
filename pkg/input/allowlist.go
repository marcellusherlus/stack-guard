package input

import (
	"encoding/json"
	"fmt"
	"os"

	"stack-guard/pkg/normalize"
	"stack-guard/pkg/types"
)

// Reads the allowlist from a JSON file and returns both the structured allowlist and a canonical set for quick lookups.
func LoadAllowlist(path string) (types.Allowlist, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return types.Allowlist{}, fmt.Errorf("read allowlist: %w", err)
	}

	var allowlist types.Allowlist
	if err := json.Unmarshal(payload, &allowlist); err != nil {
		return types.Allowlist{}, fmt.Errorf("parse allowlist json: %w", err)
	}

	allowlist.Languages = canonicalizeUnique(allowlist.Languages)
	allowlist.Frameworks = canonicalizeUnique(allowlist.Frameworks)
	allowlist.Tools = canonicalizeUnique(allowlist.Tools)

	return allowlist, nil
}

func canonicalizeUnique(values []string) []string {
	// Normalize and deduplicate once at load time so downstream lookups can use a single canonical form.
	seen := make(map[string]struct{})
	result := make([]string, 0, len(values))

	for _, value := range values {
		canonical := normalize.Canonical(value)
		if canonical == "" {
			continue
		}
		if _, exists := seen[canonical]; exists {
			continue
		}
		seen[canonical] = struct{}{}
		result = append(result, canonical)
	}

	if result == nil {
		return []string{}
	}

	return result
}
