package input

import (
	"encoding/json"
	"fmt"
	"os"

	"stack-guard/pkg/normalize"
	"stack-guard/pkg/types"
)

// Allows us to check if a technology is in the allowlist without having to iterate over a slice each time.
// We can store names canonicalized so that we catch different variations of the same technology, e.g.
// "ts":         "TypeScript",
// "typescript": "TypeScript",
// "TypeScript": "TypeScript",
// "py":         "Python",
// "python":     "Python",
type CanonicalSet map[string]struct{}

func (set CanonicalSet) Contains(name string) bool {
	canonical := normalize.Canonical(name)
	// Map lookup is O(1) on the canonical key, so aliases like "ts" and "TypeScript" match the same entry.
	_, exists := set[canonical]
	return exists
}

// Reads the allowlist from a JSON file and returns both the structured allowlist and a canonical set for quick lookups.
func LoadAllowlist(path string) (types.Allowlist, CanonicalSet, error) {
	payload, err := os.ReadFile(path)
	if err != nil {
		return types.Allowlist{}, nil, fmt.Errorf("read allowlist: %w", err)
	}

	var allowlist types.Allowlist
	if err := json.Unmarshal(payload, &allowlist); err != nil {
		return types.Allowlist{}, nil, fmt.Errorf("parse allowlist json: %w", err)
	}

	allowlist.Languages = canonicalizeUnique(allowlist.Languages)
	allowlist.Frameworks = canonicalizeUnique(allowlist.Frameworks)
	allowlist.Tools = canonicalizeUnique(allowlist.Tools)

	set := make(CanonicalSet)
	for _, value := range allowlist.Languages {
		set[value] = struct{}{}
	}
	for _, value := range allowlist.Frameworks {
		set[value] = struct{}{}
	}
	for _, value := range allowlist.Tools {
		set[value] = struct{}{}
	}

	return allowlist, set, nil
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
