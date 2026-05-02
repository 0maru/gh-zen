package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveRepositoryPath_PrefersMatchingCurrentCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	current := initTempGitRepo(t)
	runGitCommand(t, current, "remote", "add", "origin", "git@github.com:owner/repo.git")
	nested := filepath.Join(current, "nested")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	root := t.TempDir()
	other := filepath.Join(root, "owner", "repo")
	initTempGitRepoAt(t, other)
	runGitCommand(t, other, "remote", "add", "origin", "git@github.com:owner/repo.git")

	cfg := Defaults()
	cfg.Repos.Roots = []string{root}
	got := ResolveRepositoryPath(RepositoryPathOptions{
		Repo:       "owner/repo",
		Config:     cfg,
		WorkingDir: nested,
	})

	if !sameResolvedPath(t, got.Path, current) {
		t.Fatalf("expected current checkout %q to win, got %+v", current, got)
	}
}

func TestResolveRepositoryPath_FindsConfiguredRootCheckout(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	checkout := filepath.Join(root, "owner", "repo")
	initTempGitRepoAt(t, checkout)
	runGitCommand(t, checkout, "remote", "add", "origin", "https://github.com/owner/repo.git")

	cfg := Defaults()
	cfg.Repos.Roots = []string{root}
	got := ResolveRepositoryPath(RepositoryPathOptions{
		Repo:       "owner/repo",
		Config:     cfg,
		WorkingDir: t.TempDir(),
	})

	if !sameResolvedPath(t, got.Path, checkout) {
		t.Fatalf("expected checkout %q, got %+v", checkout, got)
	}
	if len(got.Diagnostics) == 0 || !strings.Contains(got.Diagnostics[0].Message, "not inside a Git repository") {
		t.Fatalf("expected current Git diagnostic before root match, got %+v", got.Diagnostics)
	}
}

func TestResolveRepositoryPath_DiagnosticsForMissingAndUnsupportedPaths(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	unsupported := filepath.Join(root, "unsupported")
	initTempGitRepoAt(t, unsupported)
	runGitCommand(t, unsupported, "remote", "add", "origin", "https://example.com/owner/repo.git")

	cfg := Defaults()
	cfg.Repos.Roots = []string{filepath.Join(root, "missing"), root}
	got := ResolveRepositoryPath(RepositoryPathOptions{
		Repo:       "owner/repo",
		Config:     cfg,
		WorkingDir: t.TempDir(),
	})

	if got.Path != "" {
		t.Fatalf("expected no checkout to resolve, got %+v", got)
	}
	if !hasDiagnosticContaining(got.Diagnostics, "repos.roots[0]", "not accessible") {
		t.Fatalf("expected missing root diagnostic, got %+v", got.Diagnostics)
	}
	if !hasDiagnosticContaining(got.Diagnostics, unsupported, "unsupported GitHub remote host") {
		t.Fatalf("expected unsupported remote diagnostic, got %+v", got.Diagnostics)
	}
	if !hasDiagnosticContaining(got.Diagnostics, "repos", "no local checkout found") {
		t.Fatalf("expected no checkout diagnostic, got %+v", got.Diagnostics)
	}
}

func TestResolveRepositoryPath_FindsNestedCheckoutInsideUnmatchedGitRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	initTempGitRepoAt(t, parent)
	runGitCommand(t, parent, "remote", "add", "origin", "https://github.com/owner/parent.git")

	nested := filepath.Join(parent, "nested", "repo")
	initTempGitRepoAt(t, nested)
	runGitCommand(t, nested, "remote", "add", "origin", "https://github.com/owner/repo.git")

	cfg := Defaults()
	cfg.Repos.Roots = []string{root}
	got := ResolveRepositoryPath(RepositoryPathOptions{
		Repo:       "owner/repo",
		Config:     cfg,
		WorkingDir: t.TempDir(),
	})

	if !sameResolvedPath(t, got.Path, nested) {
		t.Fatalf("expected nested checkout %q, got %+v", nested, got)
	}
}

func TestResolveRepositoryPath_FindsNestedCheckoutAfterParentRemoteParseError(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	root := t.TempDir()
	parent := filepath.Join(root, "parent")
	initTempGitRepoAt(t, parent)
	runGitCommand(t, parent, "remote", "add", "origin", "https://example.com/owner/parent.git")

	nested := filepath.Join(parent, "nested", "repo")
	initTempGitRepoAt(t, nested)
	runGitCommand(t, nested, "remote", "add", "origin", "https://github.com/owner/repo.git")

	cfg := Defaults()
	cfg.Repos.Roots = []string{root}
	got := ResolveRepositoryPath(RepositoryPathOptions{
		Repo:       "owner/repo",
		Config:     cfg,
		WorkingDir: t.TempDir(),
	})

	if !sameResolvedPath(t, got.Path, nested) {
		t.Fatalf("expected nested checkout %q, got %+v", nested, got)
	}
	if !hasDiagnosticContaining(got.Diagnostics, parent, "unsupported GitHub remote host") {
		t.Fatalf("expected parent remote parse diagnostic, got %+v", got.Diagnostics)
	}
}

func hasDiagnosticContaining(diagnostics []Diagnostic, path string, message string) bool {
	for _, diagnostic := range diagnostics {
		if diagnostic.Path == path && strings.Contains(diagnostic.Message, message) {
			return true
		}
	}
	return false
}

func sameResolvedPath(t *testing.T, got string, want string) bool {
	t.Helper()
	gotResolved, err := filepath.EvalSymlinks(got)
	if err != nil {
		t.Fatalf("resolve got path %q: %v", got, err)
	}
	wantResolved, err := filepath.EvalSymlinks(want)
	if err != nil {
		t.Fatalf("resolve want path %q: %v", want, err)
	}
	return gotResolved == wantResolved
}

func initTempGitRepoAt(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	runGitCommand(t, dir, "init", "-b", "main")
}
