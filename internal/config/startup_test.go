package config

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveStartupRepository_Precedence(t *testing.T) {
	resolver := func(string) (string, error) {
		return "0maru/current", nil
	}
	cfg := Defaults()
	cfg.Startup.Repo = "0maru/config"

	cases := []struct {
		name string
		opts StartupRepositoryOptions
		want StartupRepository
	}{
		{
			name: "cli wins",
			opts: StartupRepositoryOptions{
				CLIRepo:             "0maru/cli",
				Config:              cfg,
				Env:                 map[string]string{envStartupRepo: "0maru/env"},
				CurrentRepoResolver: resolver,
			},
			want: StartupRepository{Repo: "0maru/cli", Source: StartupRepoCLI},
		},
		{
			name: "env wins over config",
			opts: StartupRepositoryOptions{
				Config:              cfg,
				Env:                 map[string]string{envStartupRepo: "0maru/env"},
				CurrentRepoResolver: resolver,
			},
			want: StartupRepository{Repo: "0maru/env", Source: StartupRepoEnv},
		},
		{
			name: "config wins over current git",
			opts: StartupRepositoryOptions{
				Config:              cfg,
				Env:                 map[string]string{},
				CurrentRepoResolver: resolver,
			},
			want: StartupRepository{Repo: "0maru/config", Source: StartupRepoConfig},
		},
		{
			name: "current git fallback",
			opts: StartupRepositoryOptions{
				Config:              Defaults(),
				Env:                 map[string]string{},
				CurrentRepoResolver: resolver,
			},
			want: StartupRepository{Repo: "0maru/current", Source: StartupRepoCurrentGit},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ResolveStartupRepository(tc.opts)
			if err != nil {
				t.Fatalf("expected startup repo to resolve, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %+v, got %+v", tc.want, got)
			}
		})
	}
}

func TestResolveStartupRepository_RejectsInvalidStrongerSources(t *testing.T) {
	_, err := ResolveStartupRepository(StartupRepositoryOptions{
		CLIRepo:             "not-a-repo",
		Env:                 map[string]string{envStartupRepo: "0maru/env"},
		CurrentRepoResolver: func(string) (string, error) { return "0maru/current", nil },
	})
	if err == nil {
		t.Fatalf("expected invalid CLI repo to fail")
	}
	if !strings.Contains(err.Error(), "--repo") {
		t.Fatalf("expected error to mention --repo, got %v", err)
	}
}

func TestResolveStartupRepository_AllowsMissingCurrentGit(t *testing.T) {
	got, err := ResolveStartupRepository(StartupRepositoryOptions{
		Config:              Defaults(),
		Env:                 map[string]string{},
		CurrentRepoResolver: func(string) (string, error) { return "", os.ErrNotExist },
		AllowMissingCurrent: true,
	})
	if err != nil {
		t.Fatalf("expected missing current Git repository to be non-fatal, got %v", err)
	}
	if got.Repo != "" || got.Source != StartupRepoCurrentGit {
		t.Fatalf("expected empty current Git fallback, got %+v", got)
	}
}

func TestParseGitHubRemoteURL(t *testing.T) {
	cases := []struct {
		name    string
		remote  string
		want    string
		wantErr bool
	}{
		{name: "ssh scp", remote: "git@github.com:owner/repo.git", want: "owner/repo"},
		{name: "https git suffix", remote: "https://github.com/owner/repo.git", want: "owner/repo"},
		{name: "https no suffix", remote: "https://github.com/owner/repo", want: "owner/repo"},
		{name: "https uppercase host", remote: "https://GitHub.com/owner/repo.git", want: "owner/repo"},
		{name: "https default port", remote: "https://github.com:443/owner/repo.git", want: "owner/repo"},
		{name: "ssh url", remote: "ssh://git@github.com/owner/repo.git", want: "owner/repo"},
		{name: "ssh url default port", remote: "ssh://git@github.com:22/owner/repo.git", want: "owner/repo"},
		{name: "unsupported host", remote: "https://example.com/owner/repo.git", wantErr: true},
		{name: "unsupported scheme", remote: "http://github.com/owner/repo.git", wantErr: true},
		{name: "invalid repo", remote: "https://github.com/owner /repo.git", wantErr: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got, err := ParseGitHubRemoteURL(tc.remote)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected parse error for %q", tc.remote)
				}
				return
			}
			if err != nil {
				t.Fatalf("expected remote to parse, got %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestCurrentGitRepository_ResolvesOriginRemote(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repository")
	}

	repoDir := initTempGitRepo(t)
	runGitCommand(t, repoDir, "remote", "add", "origin", "git@github.com:owner/repo.git")

	subdir := filepath.Join(repoDir, "nested")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	got, err := CurrentGitRepository(subdir)
	if err != nil {
		t.Fatalf("expected current Git repository to resolve, got %v", err)
	}
	if got != "owner/repo" {
		t.Fatalf("expected owner/repo, got %q", got)
	}

	root, err := CurrentGitRepositoryRoot(subdir)
	if err != nil {
		t.Fatalf("expected current Git repository root to resolve, got %v", err)
	}
	wantRoot, err := filepath.EvalSymlinks(repoDir)
	if err != nil {
		t.Fatalf("resolve repo dir symlinks: %v", err)
	}
	gotRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		t.Fatalf("resolve root symlinks: %v", err)
	}
	if gotRoot != wantRoot {
		t.Fatalf("expected root %q, got %q", wantRoot, gotRoot)
	}
}

func TestCurrentGitRepository_ReportsActionableErrors(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repository")
	}

	_, err := CurrentGitRepository(t.TempDir())
	if err == nil {
		t.Fatalf("expected non-Git directory to fail")
	}
	if !strings.Contains(err.Error(), "not inside a Git repository") {
		t.Fatalf("expected non-Git error to be actionable, got %v", err)
	}

	repoDir := initTempGitRepo(t)
	runGitCommand(t, repoDir, "remote", "add", "origin", "https://example.com/owner/repo.git")
	_, err = CurrentGitRepository(repoDir)
	if err == nil {
		t.Fatalf("expected unsupported remote to fail")
	}
	if !strings.Contains(err.Error(), "unsupported GitHub remote host") {
		t.Fatalf("expected unsupported remote error, got %v", err)
	}
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGitCommand(t, dir, "init", "-b", "main")
	return dir
}

func runGitCommand(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}
