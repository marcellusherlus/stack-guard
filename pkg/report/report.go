package report

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"stack-guard/pkg/types"
)

type BuildInput struct {
	Repository    string
	Classified    []types.ClassifiedTech
	UsedAI        bool
	Assumptions   []string
	Uncertainties []string
}

// Build assembles report partitions and computes a verdict.
func Build(input BuildInput) types.Report {
	detected := append([]types.ClassifiedTech(nil), input.Classified...)
	sort.Slice(detected, func(left, right int) bool {
		if detected[left].Name == detected[right].Name {
			return detected[left].Category < detected[right].Category
		}
		return detected[left].Name < detected[right].Name
	})

	allowed := make([]types.ClassifiedTech, 0, len(detected))
	notAllowed := make([]types.ClassifiedTech, 0, len(detected))
	for _, technology := range detected {
		if technology.Allowed {
			allowed = append(allowed, technology)
			continue
		}
		notAllowed = append(notAllowed, technology)
	}

	hardViolations := 0
	softFlags := 0
	for _, technology := range notAllowed {
		if technology.Uncertain || technology.Confidence < types.ConfidenceUncertainThreshold {
			softFlags++
			continue
		}
		hardViolations++
	}

	verdict := types.VerdictCompliant
	switch {
	case hardViolations == 0 && softFlags == 0:
		verdict = types.VerdictCompliant
	case hardViolations > 0:
		verdict = types.VerdictNonCompliant
	default:
		verdict = types.VerdictUncertain
	}

	assumptions := append([]string(nil), input.Assumptions...)
	if input.UsedAI {
		assumptions = append(assumptions, "AI refinement: enabled.")
	} else {
		assumptions = append(assumptions, "AI refinement: disabled or unavailable.")
	}

	return types.Report{
		Repository:    input.Repository,
		Verdict:       verdict,
		Detected:      detected,
		Allowed:       allowed,
		NotAllowed:    notAllowed,
		Uncertainties: append([]string(nil), input.Uncertainties...),
		Assumptions:   assumptions,
		GeneratedAt:   time.Now().UTC().Format(time.RFC3339),
	}
}

func RenderText(report types.Report) string {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Repository: %s\n", report.Repository)
	fmt.Fprintf(&builder, "Verdict:    %s\n\n", strings.ToUpper(string(report.Verdict)))
	fmt.Fprintf(&builder, "Detected technologies (%d)\n", len(report.Detected))

	tabWriter := tabwriter.NewWriter(&builder, 0, 4, 2, ' ', 0)
	fmt.Fprintln(tabWriter, "  Status\tTechnology\tCategory\tConfidence\tEvidence\tNotes")
	fmt.Fprintln(tabWriter, "  ------\t----------\t--------\t----------\t--------\t-----")
	for _, technology := range report.Detected {
		marker := markerFor(technology)
		evidence := summarizeEvidence(technology.EvidenceList)
		notes := ""
		if !technology.Allowed {
			notes = "NOT ALLOWED"
		}
		if technology.Uncertain {
			if notes != "" {
				notes += ", uncertain"
			} else {
				notes = "uncertain"
			}
		}
		if technology.Notes != "" {
			if notes != "" {
				notes += "; "
			}
			notes += technology.Notes
		}
		fmt.Fprintf(tabWriter, "  %s\t%s\t%s\t%.2f\t%s\t%s\n", marker, technology.Name, technology.Category, technology.Confidence, evidence, notes)
	}
	_ = tabWriter.Flush()

	if len(report.NotAllowed) > 0 {
		builder.WriteString("\nNot allowed\n")
		for _, technology := range report.NotAllowed {
			evidence := summarizeEvidence(technology.EvidenceList)
			fmt.Fprintf(&builder, "  - %s (%.2f): %s\n", technology.Name, technology.Confidence, evidence)
		}
	}

	if len(report.Uncertainties) > 0 || len(report.Assumptions) > 0 {
		builder.WriteString("\nUncertainties & assumptions\n")
		for _, item := range report.Uncertainties {
			fmt.Fprintf(&builder, "  - %s\n", item)
		}
		for _, item := range report.Assumptions {
			fmt.Fprintf(&builder, "  - %s\n", item)
		}
	}

	return builder.String()
}

func RenderJSON(report types.Report) (string, error) {
	payload, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return "", fmt.Errorf("marshal report json: %w", err)
	}
	return string(payload), nil
}

func markerFor(technology types.ClassifiedTech) string {
	if technology.Uncertain {
		return "[?]"
	}
	if !technology.Allowed {
		return "[X]"
	}
	return "[OK]"
}

func summarizeEvidence(evidenceList []types.Evidence) string {
	if len(evidenceList) == 0 {
		return "no evidence"
	}

	parts := make([]string, 0, len(evidenceList))
	seen := make(map[string]struct{}, len(evidenceList))
	for _, evidence := range evidenceList {
		candidate := strings.TrimSpace(evidence.Source)
		if candidate == "" {
			candidate = strings.TrimSpace(evidence.Reason)
		}
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		parts = append(parts, candidate)
	}
	if len(parts) == 0 {
		return "no evidence"
	}
	sort.Strings(parts)
	return strings.Join(parts, "; ")
}
