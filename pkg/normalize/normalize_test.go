package normalize

import "testing"

func TestCanonical_AliasMapping(t *testing.T) {
	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{name: "typescript shorthand", input: "ts", expected: "TypeScript"},
		{name: "whitespace trimming", input: "  kotlin ", expected: "Kotlin"},
		{name: "node alias", input: "nodejs", expected: "Node"},
		{name: "fallback title", input: "custom-tool", expected: "Custom Tool"},
		{name: "empty value", input: "   ", expected: ""},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			actual := Canonical(testCase.input)
			if actual != testCase.expected {
				t.Fatalf("Canonical(%q) = %q, expected %q", testCase.input, actual, testCase.expected)
			}
		})
	}
}
