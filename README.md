# stack-guard

stack-guard checks a GitHub repository against an approved technology stack and reports whether the repository appears compliant.

It inspects repository files and manifests, detects technologies with explicit evidence, optionally refines classification with Claude, and outputs a human-readable report plus optional JSON.

## What it does

- Accepts a GitHub repository slug in the form org/repo.
- Loads an allowlist of approved languages, frameworks, and tools.
- Detects technologies from tree paths, file extensions, manifests, and selected file content.
- Optionally uses AI to refine confidence and uncertainty.
- Produces a verdict: compliant, non-compliant, or uncertain.

## How to run

### As Binary

```sh
#Build binary
go build -o stack-guard ./cmd
```

```sh
#Run binary
./stack-guard --allowlist allowList.json <org/repo>
# e.g.
./stack-guard --allowlist allowList.json jobrad-gmbh/odoo
```

### Direct run with go

```sh
#Run directly with go
go run cmd/main.go --allowlist allowList.json <org/repo>
# e.g.
./stack-guard --allowlist allowList.json jobrad-gmbh/odoo
```

### Input Examples

```sh
#Print usage info
go run cmd/main.go
```

```sh
#Run with JSON output file
go run cmd/main.go --allowlist allowList.json --json report.json <org/repo>
```

```sh
#Run without AI refinement
go run cmd/main.go --allowlist allowList.json --no-ai <org/repo>
```

Optional environment variables:

- GITHUB_TOKEN: recommended for higher API rate limits and private repository access.
- ANTHROPIC_API_KEY: enables AI refinement.

The app also loads variables from a local .env file at startup.

## Inputs

Required:

- Positional repository argument: org/repo
- --allowlist path to allowlist JSON

Optional flags:

- --json path to write machine-readable report
- --no-ai disable AI refinement
- --token explicit GitHub token (falls back to GITHUB_TOKEN env var if set)
- --timeout overall timeout, default 30s

Allowlist schema:

```json
{
  "languages": ["Kotlin", "Python", "TypeScript"],
  "frameworks": ["JUnit", "Spring"],
  "tools": ["Ruff", "Testcontainers"]
}
```

See [allowList.json](allowlist.json) for a ready-to-use sample.

## Approach

1. Fetch: repository default branch, recursive tree, and selected file contents.
2. Detect: deterministic rule engine maps evidence to detected technologies.
3. Classify: optional narrow AI refinement adjusts confidence, uncertainty, and notes.
4. Report: compliance verdict and structured text/JSON output.

Core principles:

- Evidence-first detection.
- Deterministic rules as baseline.
- AI as refinement only, never as primary detector.
- Graceful fallback when AI is unavailable or invalid.

## Design decisions and tradeoffs

- Static tree analysis over cloning/building:
faster and safer, but less runtime certainty.
- Single binary distribution via Go:
simple CI/CD usage and portable execution.
- Rules-as-data in pkg/rules:
explicit, testable, and easy to extend.
- Narrow AI role:
lower hallucination risk and easier reasoning.
- Three-way verdict:
uncertain is explicit instead of forcing binary pass/fail.
- Sequential fetch by default:
simpler behavior for small selected file sets.

## Limitations and known gaps

- Curated rules can miss unfamiliar ecosystems.
- TOML/Gradle/Maven checks are shallow (substring-based in places). Extra 3-Party library would be needed.
- Monorepos are reported as one flat repository view.
- Very large repositories may return truncated trees from GitHub.

## Exit codes

- 0: compliant run
- 1: non-compliant or uncertain verdict
- 2: input or usage error
- 3: runtime error

## AI usage note

AI was used while implementing this project and can optionally be used at runtime for classification refinement.
