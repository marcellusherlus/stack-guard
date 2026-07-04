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

	"stack-guard/pkg/detect"
	githubclient "stack-guard/pkg/github"
	"stack-guard/pkg/input"
)

var repositoryPattern = regexp.MustCompile(`^[\w.-]+/[\w.-]+$`)

type repositorySummary struct {
	treeEntryCount int
	detectedCount  int
}

var fetchRepositorySummary = func(ctx context.Context, token, repository string) (repositorySummary, error) {
	client := githubclient.NewClient(token)
	snapshot, err := client.FetchRepo(ctx, repository, detect.SelectFiles)
	if err != nil {
		return repositorySummary{}, err
	}
	detected := detect.Run(snapshot)
	return repositorySummary{treeEntryCount: len(snapshot.Tree), detectedCount: len(detected)}, nil
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
	exitCode := run(os.Args[1:], os.Stdout, os.Stderr)
	os.Exit(exitCode)
}

func run(args []string, stdout io.Writer, stderr io.Writer) int {
	config, err := parseConfig(args, stderr)
	if err != nil {
		fmt.Fprintf(stderr, "input error: %v\n", err)
		return 2
	}

	_, _, err = input.LoadAllowlist(config.allowlistPath)
	if err != nil {
		fmt.Fprintf(stderr, "input error: %v\n", err)
		return 2
	}

	contextWithTimeout, cancel := context.WithTimeout(context.Background(), config.timeout)
	defer cancel()

	summary, err := fetchRepositorySummary(contextWithTimeout, config.githubToken, config.repository)
	if err != nil {
		fmt.Fprintf(stderr, "runtime error: %v\n", err)
		return 3
	}

	fmt.Fprintf(stdout, "validated repository %s with allowlist %s (%d tree entries fetched, %d technologies detected)\n", config.repository, config.allowlistPath, summary.treeEntryCount, summary.detectedCount)
	return 0
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
