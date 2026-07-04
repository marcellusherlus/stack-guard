package report

import (
	"encoding/json"
	"strings"
	"testing"

	"stack-guard/pkg/types"
)

func TestBuild_VerdictCases(t *testing.T) {
	testCases := []struct {
		name     string
		input    BuildInput
		expected types.Verdict
	}{
		{
			name:     "all allowed",
			input:    BuildInput{Classified: []types.ClassifiedTech{{DetectedTech: types.DetectedTech{Name: "Go", Confidence: 0.95}, Allowed: true}}},
			expected: types.VerdictCompliant,
		},
		{
			name:     "hard violation",
			input:    BuildInput{Classified: []types.ClassifiedTech{{DetectedTech: types.DetectedTech{Name: "Ruby", Confidence: 0.9}, Allowed: false, Uncertain: false}}},
			expected: types.VerdictNonCompliant,
		},
		{
			name:     "soft only violation",
			input:    BuildInput{Classified: []types.ClassifiedTech{{DetectedTech: types.DetectedTech{Name: "Ruby", Confidence: 0.4}, Allowed: false, Uncertain: true}}},
			expected: types.VerdictUncertain,
		},
	}

	for _, testCase := range testCases {
		t.Run(testCase.name, func(t *testing.T) {
			report := Build(testCase.input)
			if report.Verdict != testCase.expected {
				t.Fatalf("Build() verdict = %s, expected %s", report.Verdict, testCase.expected)
			}
		})
	}
}

func TestRenderJSON_RoundTrip(t *testing.T) {
	input := BuildInput{
		Repository: "org/repo",
		Classified: []types.ClassifiedTech{{DetectedTech: types.DetectedTech{Name: "Go", Confidence: 0.95}, Allowed: true}},
	}
	report := Build(input)

	encoded, err := RenderJSON(report)
	if err != nil {
		t.Fatalf("RenderJSON returned error: %v", err)
	}

	var decoded types.Report
	if err := json.Unmarshal([]byte(encoded), &decoded); err != nil {
		t.Fatalf("json.Unmarshal returned error: %v", err)
	}

	if decoded.Repository != report.Repository {
		t.Fatalf("decoded repository = %q, expected %q", decoded.Repository, report.Repository)
	}
	if decoded.Verdict != report.Verdict {
		t.Fatalf("decoded verdict = %s, expected %s", decoded.Verdict, report.Verdict)
	}
}

func TestRenderText_IncludesMarkersAndRepository(t *testing.T) {
	report := Build(BuildInput{
		Repository: "org/repo",
		Classified: []types.ClassifiedTech{
			{DetectedTech: types.DetectedTech{Name: "Go", Confidence: 0.95, Category: types.CategoryLanguage}, Allowed: true},
			{DetectedTech: types.DetectedTech{Name: "Ruby", Confidence: 0.4, Category: types.CategoryLanguage}, Allowed: false, Uncertain: true},
		},
		Uncertainties: []string{"Example uncertainty"},
	})

	rendered := RenderText(report)
	if !strings.Contains(rendered, "Repository: org/repo") {
		t.Fatalf("expected repository line, got %q", rendered)
	}
	if !strings.Contains(rendered, "[OK]") {
		t.Fatalf("expected [OK] marker, got %q", rendered)
	}
	if !strings.Contains(rendered, "[?]") {
		t.Fatalf("expected [?] marker, got %q", rendered)
	}
}
