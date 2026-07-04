package classify

import (
	"context"
	"errors"
	"testing"

	"stack-guard/pkg/types"
)

type fakeCompleter struct {
	completion string
	err        error
	calls      int
}

func (fake *fakeCompleter) Complete(_ context.Context, _, _ string) (string, error) {
	fake.calls++
	if fake.err != nil {
		return "", fake.err
	}
	return fake.completion, nil
}

func TestClassifier_DisabledFallsBack(t *testing.T) {
	classifier := NewClassifier(&fakeCompleter{completion: `{}`}, true)
	detected := []types.DetectedTech{{Name: "Ruby", Category: types.CategoryLanguage, Confidence: 0.4}}
	allowlist := types.Allowlist{Languages: []string{"Go"}}

	classified, uncertainties, usedAI := classifier.Classify(context.Background(), detected, allowlist)
	if usedAI {
		t.Fatal("expected usedAI=false when classifier is disabled")
	}
	if len(classified) != 1 {
		t.Fatalf("expected one result, got %d", len(classified))
	}
	if !classified[0].Uncertain {
		t.Fatal("expected low confidence tech to be uncertain in fallback")
	}
	if len(uncertainties) == 0 {
		t.Fatal("expected fallback uncertainty note")
	}
}

func TestClassifier_AppliesAIOverridesAndDropsUnknownTech(t *testing.T) {
	fake := &fakeCompleter{completion: `{
		"technologies": [
			{"name":"TypeScript","category":"language","confidence":0.97,"uncertain":false,"notes":"Primary language."},
			{"name":"ImaginaryTech","category":"tool","confidence":0.90,"uncertain":false,"notes":"ignored"}
		],
		"uncertainties": ["Minor uncertainty"]
	}`}
	classifier := NewClassifier(fake, false)

	detected := []types.DetectedTech{{Name: "TypeScript", Category: types.CategoryLanguage, Confidence: 0.7}}
	allowlist := types.Allowlist{Languages: []string{"TypeScript"}}

	classified, uncertainties, usedAI := classifier.Classify(context.Background(), detected, allowlist)
	if !usedAI {
		t.Fatal("expected usedAI=true")
	}
	if fake.calls != 1 {
		t.Fatalf("expected one AI call, got %d", fake.calls)
	}
	if len(classified) != 1 {
		t.Fatalf("expected one classified result, got %d", len(classified))
	}
	if classified[0].Confidence != 0.97 {
		t.Fatalf("expected AI confidence override, got %.2f", classified[0].Confidence)
	}
	if classified[0].Notes == "" {
		t.Fatal("expected AI notes to be propagated")
	}
	if len(uncertainties) != 1 || uncertainties[0] != "Minor uncertainty" {
		t.Fatalf("unexpected uncertainties: %#v", uncertainties)
	}
}

func TestClassifier_MalformedJSONFallsBack(t *testing.T) {
	classifier := NewClassifier(&fakeCompleter{completion: `{"technologies":`}, false)
	detected := []types.DetectedTech{{Name: "Go", Category: types.CategoryLanguage, Confidence: 0.95}}
	allowlist := types.Allowlist{Languages: []string{"Go"}}

	classified, _, usedAI := classifier.Classify(context.Background(), detected, allowlist)
	if usedAI {
		t.Fatal("expected usedAI=false on malformed completion")
	}
	if len(classified) != 1 || !classified[0].Allowed {
		t.Fatalf("expected fallback classified result, got %#v", classified)
	}
}

func TestClassifier_AIFailureFallsBack(t *testing.T) {
	classifier := NewClassifier(&fakeCompleter{err: errors.New("upstream failed")}, false)
	detected := []types.DetectedTech{{Name: "Go", Category: types.CategoryLanguage, Confidence: 0.95}}
	allowlist := types.Allowlist{Languages: []string{"Go"}}

	classified, _, usedAI := classifier.Classify(context.Background(), detected, allowlist)
	if usedAI {
		t.Fatal("expected usedAI=false on AI error")
	}
	if len(classified) != 1 || !classified[0].Allowed {
		t.Fatalf("expected fallback classified result, got %#v", classified)
	}
}

func TestClassifier_ParsesFencedJSONCompletion(t *testing.T) {
	classifier := NewClassifier(&fakeCompleter{completion: "```json\n{\"technologies\":[{\"name\":\"TypeScript\",\"category\":\"language\",\"confidence\":0.91,\"uncertain\":false,\"notes\":\"from fenced json\"}],\"uncertainties\":[]}\n```"}, false)
	detected := []types.DetectedTech{{Name: "TypeScript", Category: types.CategoryLanguage, Confidence: 0.70}}
	allowlist := types.Allowlist{Languages: []string{"TypeScript"}}

	classified, uncertainties, usedAI := classifier.Classify(context.Background(), detected, allowlist)
	if !usedAI {
		t.Fatal("expected usedAI=true for fenced JSON completion")
	}
	if len(uncertainties) != 0 {
		t.Fatalf("expected no uncertainties, got %#v", uncertainties)
	}
	if len(classified) != 1 {
		t.Fatalf("expected one classified result, got %d", len(classified))
	}
	if classified[0].Confidence != 0.91 {
		t.Fatalf("expected confidence 0.91, got %.2f", classified[0].Confidence)
	}
}
