package types

type Category string

const (
	CategoryLanguage  Category = "language"
	CategoryFramework Category = "framework"
	CategoryTool      Category = "tool"
	CategoryUnknown   Category = "unknown"
)

// Evidence explains why a technology was detected.
type Evidence struct {
	Source     string  `json:"source"`
	Reason     string  `json:"reason"`
	Confidence float64 `json:"confidence"`
}

// DetectedTech is the deterministic output from the detection stage.
type DetectedTech struct {
	Name         string     `json:"name"`
	Category     Category   `json:"category"`
	EvidenceList []Evidence `json:"evidence"`
	Confidence   float64    `json:"confidence"`
}

// Allowlist contains approved technologies by category.
type Allowlist struct {
	Languages  []string `json:"languages"`
	Frameworks []string `json:"frameworks"`
	Tools      []string `json:"tools"`
}

// ClassifiedTech augments deterministic detection with classification metadata.
type ClassifiedTech struct {
	DetectedTech
	Allowed   bool   `json:"allowed"`
	Uncertain bool   `json:"uncertain"`
	Notes     string `json:"notes,omitempty"`
}

// Confidence thresholds used across detection, classification, and reporting.
const (
	ConfidenceUncertainThreshold = 0.5  // below this a finding is treated as uncertain
	ConfidenceMax                = 0.99 // cap to avoid implying absolute certainty
)

type Verdict string

const (
	VerdictCompliant    Verdict = "compliant"
	VerdictNonCompliant Verdict = "non-compliant"
	VerdictUncertain    Verdict = "uncertain"
)

type Report struct {
	Repository    string           `json:"repository"`
	Verdict       Verdict          `json:"verdict"`
	Detected      []ClassifiedTech `json:"detected"`
	Allowed       []ClassifiedTech `json:"allowed"`
	NotAllowed    []ClassifiedTech `json:"notAllowed"`
	Uncertainties []string         `json:"uncertainties"`
	Assumptions   []string         `json:"assumptions"`
	GeneratedAt   string           `json:"generatedAt"`
}
