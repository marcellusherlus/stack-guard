package detect

import (
	"math"
	"path/filepath"
	"sort"

	"stack-guard/pkg/github"
	"stack-guard/pkg/normalize"
	"stack-guard/pkg/rules"
	"stack-guard/pkg/types"
)

// SelectFiles returns the rule-required files that exist in the fetched tree.
func SelectFiles(allPaths []string) []string {
	existing := make(map[string]struct{}, len(allPaths))
	for _, filePath := range allPaths {
		existing[filePath] = struct{}{}
	}

	selected := make(map[string]struct{})
	for _, rule := range rules.All() {
		for _, filePath := range rule.NeedsContentOf {
			if _, ok := existing[filePath]; ok {
				selected[filePath] = struct{}{}
			}
		}
	}

	result := make([]string, 0, len(selected))
	for filePath := range selected {
		result = append(result, filePath)
	}
	sort.Strings(result)
	return result
}

// Run executes the rule table and aggregates evidence by canonical technology.
func Run(snapshot github.RepoSnapshot) []types.DetectedTech {
	matchContext := rules.MatchContext{
		Paths:          blobPaths(snapshot.Tree),
		ExtensionCount: extensionCount(snapshot.Tree),
		FileContents:   snapshot.Files,
	}

	aggregated := make(map[string]types.DetectedTech)
	for _, rule := range rules.All() {
		evidence := rule.Match(matchContext)
		if evidence == nil {
			continue
		}

		canonicalName := normalize.Canonical(rule.Tech)
		detected, exists := aggregated[canonicalName]
		if !exists {
			detected = types.DetectedTech{Name: canonicalName, Category: rule.Category}
		}
		detected.Confidence = aggregateConfidence(detected.Confidence, evidence.Confidence)
		detected.EvidenceList = append(detected.EvidenceList, *evidence)
		aggregated[canonicalName] = detected
	}

	result := make([]types.DetectedTech, 0, len(aggregated))
	for _, detected := range aggregated {
		sort.Slice(detected.EvidenceList, func(left, right int) bool {
			if detected.EvidenceList[left].Source == detected.EvidenceList[right].Source {
				return detected.EvidenceList[left].Reason < detected.EvidenceList[right].Reason
			}
			return detected.EvidenceList[left].Source < detected.EvidenceList[right].Source
		})
		result = append(result, detected)
	}

	sort.Slice(result, func(left, right int) bool {
		if result[left].Name == result[right].Name {
			return result[left].Category < result[right].Category
		}
		return result[left].Name < result[right].Name
	})

	return result
}

func blobPaths(tree []github.TreeEntry) []string {
	paths := make([]string, 0, len(tree))
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		paths = append(paths, entry.Path)
	}
	sort.Strings(paths)
	return paths
}

func extensionCount(tree []github.TreeEntry) map[string]int {
	counts := make(map[string]int)
	for _, entry := range tree {
		if entry.Type != "blob" {
			continue
		}
		extension := filepath.Ext(entry.Path)
		if extension == "" {
			continue
		}
		counts[extension]++
	}
	return counts
}

func aggregateConfidence(existing, next float64) float64 {
	if existing <= 0 {
		return capConfidence(next)
	}
	combined := 1 - ((1 - existing) * (1 - next))
	return capConfidence(combined)
}

func capConfidence(value float64) float64 {
	if value > 0.99 {
		return 0.99
	}
	return math.Round(value*100) / 100
}
