package config

import (
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"strings"
)

const envStartupRepo = "GH_ZEN_REPO"

// StartupRepoSource identifies where the startup repository came from.
type StartupRepoSource string

const (
	StartupRepoCLI        StartupRepoSource = "cli"
	StartupRepoEnv        StartupRepoSource = "env"
	StartupRepoConfig     StartupRepoSource = "config"
	StartupRepoCurrentGit StartupRepoSource = "current_git"
)

// StartupRepository is the resolved startup repository and its source.
type StartupRepository struct {
	Repo   string
	Source StartupRepoSource
}

// CurrentRepoResolver resolves owner/repo from a local working directory.
type CurrentRepoResolver func(workingDir string) (string, error)

// StartupRepositoryOptions configures startup repository resolution.
type StartupRepositoryOptions struct {
	CLIRepo             string
	Config              Config
	WorkingDir          string
	Env                 map[string]string
	CurrentRepoResolver CurrentRepoResolver
	AllowMissingCurrent bool
}

// ResolveStartupRepository applies the ADR 0007 startup repository precedence.
func ResolveStartupRepository(options StartupRepositoryOptions) (StartupRepository, error) {
	if options.CLIRepo != "" {
		return startupRepoFromValue(options.CLIRepo, StartupRepoCLI, "--repo")
	}
	if envRepo := startupRepoFromEnv(options.Env); envRepo != "" {
		return startupRepoFromValue(envRepo, StartupRepoEnv, envStartupRepo)
	}
	if options.Config.Startup.Repo != "" {
		return startupRepoFromValue(options.Config.Startup.Repo, StartupRepoConfig, "startup.repo")
	}

	resolver := options.CurrentRepoResolver
	if resolver == nil {
		resolver = CurrentGitRepository
	}
	repo, err := resolver(options.WorkingDir)
	if err != nil {
		if options.AllowMissingCurrent {
			return StartupRepository{Source: StartupRepoCurrentGit}, nil
		}
		return StartupRepository{}, fmt.Errorf("resolve startup repository from current Git repository: %w", err)
	}
	return startupRepoFromValue(repo, StartupRepoCurrentGit, "current Git repository")
}

// CurrentGitRepository resolves owner/repo from the current Git repository's origin remote.
func CurrentGitRepository(workingDir string) (string, error) {
	root, err := CurrentGitRepositoryRoot(workingDir)
	if err != nil {
		return "", err
	}
	remote, err := runGit(root, "remote", "get-url", "origin")
	if err != nil {
		return "", fmt.Errorf("origin remote is not configured for %q", root)
	}
	repo, err := ParseGitHubRemoteURL(remote)
	if err != nil {
		return "", err
	}
	return repo, nil
}

// CurrentGitRepositoryRoot resolves the root path for the current Git checkout.
func CurrentGitRepositoryRoot(workingDir string) (string, error) {
	if workingDir == "" {
		var err error
		workingDir, err = os.Getwd()
		if err != nil {
			return "", fmt.Errorf("resolve working directory: %w", err)
		}
	}

	root, err := runGit(workingDir, "rev-parse", "--show-toplevel")
	if err != nil {
		return "", fmt.Errorf("%q is not inside a Git repository", workingDir)
	}
	return root, nil
}

// ParseGitHubRemoteURL converts supported GitHub remote URLs into owner/repo.
func ParseGitHubRemoteURL(raw string) (string, error) {
	remote := strings.TrimSpace(raw)
	if remote == "" {
		return "", fmt.Errorf("GitHub remote URL is empty")
	}

	var repoPath string
	switch {
	case strings.HasPrefix(remote, "git@github.com:"):
		repoPath = strings.TrimPrefix(remote, "git@github.com:")
	default:
		parsed, err := url.Parse(remote)
		if err != nil {
			return "", fmt.Errorf("parse GitHub remote URL %q: %w", raw, err)
		}
		if !strings.EqualFold(parsed.Hostname(), "github.com") {
			return "", fmt.Errorf("unsupported GitHub remote host %q", parsed.Host)
		}
		switch parsed.Scheme {
		case "https", "ssh":
			repoPath = strings.TrimPrefix(parsed.Path, "/")
		default:
			return "", fmt.Errorf("unsupported GitHub remote scheme %q", parsed.Scheme)
		}
	}

	repo := strings.TrimSuffix(repoPath, ".git")
	if !isRepoFullName(repo) {
		return "", fmt.Errorf("GitHub remote URL %q does not contain a valid owner/repo", raw)
	}
	return repo, nil
}

func startupRepoFromValue(value string, source StartupRepoSource, label string) (StartupRepository, error) {
	if !isRepoFullName(value) {
		return StartupRepository{}, fmt.Errorf("%s must use owner/repo format", label)
	}
	return StartupRepository{Repo: value, Source: source}, nil
}

func startupRepoFromEnv(env map[string]string) string {
	if env != nil {
		return env[envStartupRepo]
	}
	return os.Getenv(envStartupRepo)
}

func runGit(workingDir string, args ...string) (string, error) {
	cmd := exec.Command("git", append([]string{"-C", workingDir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}
