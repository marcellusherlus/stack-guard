package classify

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"stack-guard/pkg/normalize"
	"stack-guard/pkg/types"
)

const systemPrompt = "You are a precise JSON API. Respond with raw JSON only — no markdown, no code fences, no explanation before or after. The response must start with { and end with }. Use exactly two top-level keys: technologies (array) and uncertainties (array of strings). Never invent technologies not present in the input. Prefer uncertain=true over guessing."

type completer interface {
	Complete(ctx context.Context, systemPrompt, userPayload string) (string, error)
}

// Classifier refines deterministic detections and degrades safely when AI is unavailable.
type Classifier struct {
	ai       completer
	disabled bool
}

func NewClassifier(ai completer, disabled bool) *Classifier {
	return &Classifier{ai: ai, disabled: disabled}
}

type payload struct {
	Detected  []types.DetectedTech `json:"detected"`
	Allowlist types.Allowlist      `json:"allowlist"`
}

type aiResponse struct {
	Technologies  []aiTechnology `json:"technologies"`
	Uncertainties []string       `json:"uncertainties"`
}

type aiTechnology struct {
	Name       string  `json:"name"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
	Uncertain  bool    `json:"uncertain"`
	Notes      string  `json:"notes"`
}

// Classify returns refined technologies, surfaced uncertainties, and whether AI was used.
func (classifier *Classifier) Classify(ctx context.Context, detected []types.DetectedTech, allowlist types.Allowlist) ([]types.ClassifiedTech, []string, bool) {
	if classifier.disabled || classifier.ai == nil || isNilCompleter(classifier.ai) {
		classified := fallbackClassify(detected, allowlist)
		return classified, nil, false
	}

	requestPayload, err := json.Marshal(payload{Detected: detected, Allowlist: allowlist})
	if err != nil {
		return fallbackWithReason(detected, allowlist, err)
	}

	completion, err := classifier.ai.Complete(ctx, systemPrompt, string(requestPayload))
	if err != nil {
		return fallbackWithReason(detected, allowlist, err)
	}

	response, err := parseAIResponse(completion)
	if err != nil {
		return fallbackWithReason(detected, allowlist, err)
	}

	classified := fallbackClassify(detected, allowlist)
	indexByName := make(map[string]int, len(classified))
	for index := range classified {
		indexByName[normalize.Canonical(classified[index].Name)] = index
	}

	for _, refined := range response.Technologies {
		canonical := normalize.Canonical(refined.Name)
		index, exists := indexByName[canonical]
		if !exists {
			continue
		}

		if refined.Category != "" {
			classified[index].Category = parseCategory(refined.Category, classified[index].Category)
		}
		if refined.Confidence > 0 {
			classified[index].Confidence = clampConfidence(refined.Confidence)
		}
		classified[index].Uncertain = refined.Uncertain || classified[index].Confidence < 0.5
		classified[index].Notes = strings.TrimSpace(refined.Notes)
	}

	return classified, response.Uncertainties, true
}

func fallbackClassify(detected []types.DetectedTech, allowlist types.Allowlist) []types.ClassifiedTech {
	allowedSet := allowlistSet(allowlist)
	result := make([]types.ClassifiedTech, 0, len(detected))
	for _, detectedTech := range detected {
		confidence := clampConfidence(detectedTech.Confidence)
		result = append(result, types.ClassifiedTech{
			DetectedTech: types.DetectedTech{
				Name:         detectedTech.Name,
				Category:     detectedTech.Category,
				EvidenceList: detectedTech.EvidenceList,
				Confidence:   confidence,
			},
			Allowed:   allowedSet[normalize.Canonical(detectedTech.Name)],
			Uncertain: confidence < 0.5,
		})
	}
	return result
}

func fallbackWithReason(detected []types.DetectedTech, allowlist types.Allowlist, reason error) ([]types.ClassifiedTech, []string, bool) {
	classified := fallbackClassify(detected, allowlist)
	return classified, []string{fmt.Sprintf("AI fallback reason: %v", reason)}, false
}

func allowlistSet(allowlist types.Allowlist) map[string]bool {
	set := make(map[string]bool)
	for _, item := range allowlist.Languages {
		set[normalize.Canonical(item)] = true
	}
	for _, item := range allowlist.Frameworks {
		set[normalize.Canonical(item)] = true
	}
	for _, item := range allowlist.Tools {
		set[normalize.Canonical(item)] = true
	}
	return set
}

func parseCategory(category string, fallback types.Category) types.Category {
	switch strings.ToLower(strings.TrimSpace(category)) {
	case string(types.CategoryLanguage):
		return types.CategoryLanguage
	case string(types.CategoryFramework):
		return types.CategoryFramework
	case string(types.CategoryTool):
		return types.CategoryTool
	case string(types.CategoryUnknown):
		return types.CategoryUnknown
	default:
		return fallback
	}
}

func clampConfidence(confidence float64) float64 {
	if confidence < 0 {
		return 0
	}
	if confidence > 0.99 {
		return 0.99
	}
	return confidence
}

func parseAIResponse(completion string) (aiResponse, error) {
	trimmed := strings.TrimSpace(completion)

	// decode directly — json.Decoder stops after the first complete
	// JSON value and tolerates trailing text that some models append.
	var response aiResponse
	if err := json.NewDecoder(strings.NewReader(trimmed)).Decode(&response); err == nil {
		return response, nil
	}

	// Strip a markdown code fence (```[lang]\n ... \n```) and retry.
	if strings.HasPrefix(trimmed, "```") {
		if newline := strings.Index(trimmed, "\n"); newline >= 0 {
			inner := strings.TrimSpace(trimmed[newline+1:])
			if fenceEnd := strings.LastIndex(inner, "```"); fenceEnd >= 0 {
				inner = strings.TrimSpace(inner[:fenceEnd])
			}
			if err := json.NewDecoder(strings.NewReader(inner)).Decode(&response); err == nil {
				return response, nil
			}
		}
	}

	// Extract the outermost {...} and decode.
	firstBrace := strings.Index(trimmed, "{")
	if firstBrace >= 0 {
		if err := json.NewDecoder(strings.NewReader(trimmed[firstBrace:])).Decode(&response); err == nil {
			return response, nil
		}
	}

	// Return the original decode error for diagnostics.
	var diagErr error
	if err := json.NewDecoder(strings.NewReader(trimmed)).Decode(&response); err != nil {
		diagErr = err
	}
	return aiResponse{}, diagErr
}

func isNilCompleter(ai completer) bool {
	if ai == nil {
		return true
	}
	value := reflect.ValueOf(ai)
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}

func promptSummary(detected []types.DetectedTech) string {
	return fmt.Sprintf("%d technologies detected", len(detected))
}
