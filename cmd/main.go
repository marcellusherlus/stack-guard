package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"stack-guard/pkg/classify"
	"stack-guard/pkg/claudeapi"
	"stack-guard/pkg/detect"
	githubclient "stack-guard/pkg/github"
	"stack-guard/pkg/input"
	"stack-guard/pkg/report"
	"stack-guard/pkg/types"
)

var repositoryPattern = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

var fetchRepositoryReport = func(ctx context.Context, token, repository string, allowlist types.Allowlist, disableAI bool) (types.Report, error) {
	client := githubclient.NewClient(token)
	snapshot, err := client.FetchRepo(ctx, repository, detect.SelectFiles)
	if err != nil {
		return types.Report{}, err
	}

	detected := detect.Run(snapshot)

	apiKey := strings.TrimSpace(os.Getenv("ANTHROPIC_API_KEY"))
	var classifier *classify.Classifier

	if apiKey == "" {
		classifier = classify.NewClassifier(nil, disableAI)
	} else {
		classifier = classify.NewClassifier(claudeapi.NewClient(apiKey), disableAI)
	}

	classified, uncertainties, usedAI := classifier.Classify(ctx, detected, allowlist)

	assumptions := []string{
		"Detection is static; no build was executed.",
		"Validation runs at language/framework/tool level, not individual libraries.",
	}
	if snapshot.Truncated {
		assumptions = append(assumptions, "GitHub tree was truncated; detection coverage may be incomplete.")
	} else {
		assumptions = append(assumptions, "GitHub tree was complete.")
	}

	reportValue := report.Build(report.BuildInput{
		Repository:    repository,
		Classified:    classified,
		UsedAI:        usedAI,
		Assumptions:   assumptions,
		Uncertainties: uncertainties,
	})

	return reportValue, nil
}

type cliConfig struct {
	repository     string
	allowlistPath  string
	jsonOutputPath string
	disableAI      bool
	githubToken    string
	timeout        time.Duration
}

func main() {
	err := godotenv.Load()
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		fmt.Fprintf(os.Stderr, "warning: failed to load .env: %v\n", err)
	}

	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	config, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "input error: %v\n", err)
		return 2
	}

	allowlist, _, err := input.LoadAllowlist(config.allowlistPath)
	if err != nil {
		fmt.Fprintf(stderr, "input error: %v\n", err)
		return 2
	}

	contextWithTimeout, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	reportValue, err := fetchRepositoryReport(contextWithTimeout, config.githubToken, config.repository, allowlist, config.disableAI)
	if err != nil {
		fmt.Fprintf(stderr, "runtime error: %v\n", err)
		return 3
	}

	if config.jsonOutputPath != "" {
		payload, err := report.RenderJSON(reportValue)
		if err != nil {
			fmt.Fprintf(stderr, "runtime error: %v\n", err)
			return 3
		}
		if err := os.WriteFile(config.jsonOutputPath, []byte(payload), 0o600); err != nil {
			fmt.Fprintf(stderr, "runtime error: write json report: %v\n", err)
			return 3
		}
	}

	fmt.Fprint(stdout, report.RenderText(reportValue))

	if reportValue.Verdict == types.VerdictCompliant {
		return 0
	}
	if reportValue.Verdict == types.VerdictUncertain {
		fmt.Fprintln(stderr, "warning: uncertain verdict requires manual review")
	}
	return 1
}

func parseConfig(args []string, stderr io.Writer) (cliConfig, error) {
	flagSet := flag.NewFlagSet("stack-guard", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	allowlistPath := flagSet.String("allowlist", "", "path to allowlist JSON")
	jsonOutputPath := flagSet.String("json", "", "write JSON report to this path")
	disableAI := flagSet.Bool("no-ai", false, "disable AI refinement")
	githubToken := flagSet.String("token", "", "github token (falls back to GITHUB_TOKEN)")
	timeout := flagSet.Duration("timeout", 30*time.Second, "overall timeout")

	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			printUsage(stderr)
			return cliConfig{}, errors.New("help requested")
		}
		return cliConfig{}, err
	}

	positionals := flagSet.Args()
	if len(positionals) != 1 {
		printUsage(stderr)
		return cliConfig{}, errors.New("repository positional argument is required")
	}

	repository := strings.TrimSpace(positionals[0])
	if !repositoryPattern.MatchString(repository) {
		return cliConfig{}, fmt.Errorf("invalid repository %q, expected org/repo", repository)
	}

	if strings.TrimSpace(*allowlistPath) == "" {
		return cliConfig{}, errors.New("--allowlist is required")
	}

	token := strings.TrimSpace(*githubToken)
	if token == "" {
		token = strings.TrimSpace(os.Getenv("GITHUB_TOKEN"))
	}

	return cliConfig{
		repository:     repository,
		allowlistPath:  *allowlistPath,
		jsonOutputPath: *jsonOutputPath,
		disableAI:      *disableAI,
		githubToken:    token,
		timeout:        *timeout,
	}, nil
}

func printUsage(stderr io.Writer) {
	fmt.Fprintln(stderr, "usage: stack-guard --allowlist <path> [options] <org/repo>")
	fmt.Fprintln(stderr, "options:")
	fmt.Fprintln(stderr, "  --allowlist <path>   path to allowlist JSON (required)")
	fmt.Fprintln(stderr, "  --json <path>        write JSON report to this path")
	fmt.Fprintln(stderr, "  --no-ai              disable AI refinement")
	fmt.Fprintln(stderr, "  --token <token>      GitHub token (or use GITHUB_TOKEN)")
	fmt.Fprintln(stderr, "  --timeout <duration> overall timeout (default 30s)")
}
