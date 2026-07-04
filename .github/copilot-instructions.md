# stack-guard — AI Project Overview

## Purpose

`stack-guard` is a Go CLI tool that checks a GitHub repository against an approved technology stack allowlist and reports whether the repo appears compliant. It detects technologies via a deterministic rule engine, optionally refines results using Claude (Anthropic), and outputs a human-readable report plus optional JSON.

## Pipeline

```
Input (org/repo + allowlist.json)
  → Fetch     (GitHub REST: recursive file tree + selected file contents)
  → Detect    (deterministic rules: manifests/lockfiles/configs/extensions → candidates)
  → Classify  (Claude, narrow: normalize names, disambiguate, flag uncertainty)
  → Report    (compliance verdict + evidence + uncertainties; text + JSON)
```

## Repository Structure

```
cmd/
  main.go              # entrypoint: flag parsing → orchestration
  main_test.go
pkg/
  types/               # shared domain types (Category, Evidence, DetectedTech, Allowlist, ClassifiedTech, Report, Verdict)
  input/               # load + validate allowlist JSON, normalize names
  github/              # concrete *Client: fetch tree + selected file contents via GitHub REST API
  detect/              # deterministic rule engine: runs all rules, aggregates evidence by canonical tech name
  rules/               # rule data table (manifest, path-prefix, extension, content-match rules)
  normalize/           # canonical tech-name mapping (e.g. "typescript" → "TypeScript")
  claudeapi/           # concrete *Client wrapping anthropic-sdk-go; exposes Complete(ctx, system, user) string
  classify/            # narrow Claude refinement + fallback; defines consumer-side completer interface
  report/              # compliance logic + text/JSON rendering
plan/                  # design documents (architecture, feature specs)
allowList.json         # sample allowlist
```

## Key Design Principles

- **Evidence-first detection.** Every `DetectedTech` carries the `Evidence` (source file, reason, confidence) that triggered it.
- **Deterministic core.** Detection is rule-based and reproducible. AI is used only for judgment/refinement on ambiguous results.
- **Graceful degradation.** With `--no-ai` or on API failure the tool still produces a report from raw detection, marked lower-confidence.
- **Standard library first.** `net/http`, `encoding/json`, `flag`, `context`, `testing`. Minimal third-party runtime dependencies. Ships as a single static binary.
- **Interfaces defined by consumers.** Concrete types (`*github.Client`, `*claudeapi.Client`) are returned as concrete structs. Consumer packages declare the small interface they need. Producers never ship an interface "just in case."

## Core Types (`pkg/types`)

```go
type Category string  // "language" | "framework" | "tool" | "unknown"

type Evidence struct {
    Source     string  // e.g. "build.gradle.kts" or "*.py (42 files)"
    Reason     string  // human-readable why
    Confidence float64 // 0..1
}

type DetectedTech struct {
    Name         string     // normalized, e.g. "Kotlin"
    Category     Category
    EvidenceList []Evidence
    Confidence   float64    // aggregate 0..1
}

type Allowlist struct {
    Languages  []string
    Frameworks []string
    Tools      []string
}

type ClassifiedTech struct {
    DetectedTech
    Allowed   bool
    Uncertain bool
    Notes     string
}

type Verdict string  // "compliant" | "non-compliant" | "uncertain"

type Report struct {
    Repository    string
    Verdict       Verdict
    Detected      []ClassifiedTech
    Allowed       []ClassifiedTech
    NotAllowed    []ClassifiedTech
    Uncertainties []string
    Assumptions   []string
    GeneratedAt   string
}
```

## CLI Flags

| Flag                   | Description                                       |
| ---------------------- | ------------------------------------------------- |
| `<org/repo>`           | (positional) GitHub repository slug               |
| `--allowlist <path>`   | Path to allowlist JSON (required)                 |
| `--json <path>`        | Write machine-readable JSON report to file        |
| `--no-ai`              | Disable Claude refinement                         |
| `--token <token>`      | GitHub PAT (falls back to `GITHUB_TOKEN` env var) |
| `--timeout <duration>` | Overall timeout, default `30s`                    |

## Environment Variables

- `GITHUB_TOKEN` — GitHub personal access token (higher rate limits, private repos)
- `ANTHROPIC_API_KEY` — Enables Claude AI refinement

## Naming & Style Constraints

- **Descriptive identifiers**: `repository`, `allowlist`, `evidenceList`, `detectedTech` — not `r`, `a`, `e`, `t`. Short receiver names are the one accepted idiom (`func (client *Client)`).
- **Receiver methods** where a value has behavior tied to its state (clients, report). **Free functions** for pure transforms (rule matching, rendering).
- Errors wrapped with `fmt.Errorf("...: %w", err)`; no panics in library code.
- No goroutines/`WaitGroup` unless a measured need is demonstrated — sequential fetching is fine for this workload.

## Dependencies

- `github.com/joho/godotenv` — loads `.env` file at startup
- `github.com/anthropics/anthropic-sdk-go` — Claude API client
- Standard library for everything else (HTTP, JSON, flags, testing)
