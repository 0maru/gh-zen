package localrepo

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestParseWorktreeListPorcelain(t *testing.T) {
	output := strings.TrimSpace(`
worktree /repo
HEAD 1111111111111111111111111111111111111111
branch refs/heads/main

worktree /repo-detached
HEAD 2222222222222222222222222222222222222222
detached

worktree /repo-missing
HEAD 3333333333333333333333333333333333333333
branch refs/heads/feature
prunable gitdir file points to non-existent location
`)

	got, err := ParseWorktreeListPorcelain(output)
	if err != nil {
		t.Fatalf("expected porcelain output to parse, got %v", err)
	}

	want := []Worktree{
		{Path: "/repo", Head: "1111111111111111111111111111111111111111", Branch: "main"},
		{Path: "/repo-detached", Head: "2222222222222222222222222222222222222222", Detached: true},
		{
			Path:           "/repo-missing",
			Head:           "3333333333333333333333333333333333333333",
			Branch:         "feature",
			Prunable:       true,
			PrunableReason: "gitdir file points to non-existent location",
		},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected worktrees %+v, got %+v", want, got)
	}
}

func TestParseWorktreeListPorcelain_RejectsMalformedBlocks(t *testing.T) {
	_, err := ParseWorktreeListPorcelain("HEAD abc")
	if err == nil {
		t.Fatalf("expected missing worktree path to fail")
	}
}

func TestPorcelainStatusEntries(t *testing.T) {
	got := porcelainStatusEntries(" M file.go\n?? new.go\n")
	want := []string{" M file.go", "?? new.go"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected status entries %#v, got %#v", want, got)
	}
	if entries := porcelainStatusEntries(""); len(entries) != 0 {
		t.Fatalf("expected empty status entries, got %#v", entries)
	}
}

func TestParseBranchRefs(t *testing.T) {
	output := strings.TrimSpace(`
refs/heads/main
refs/heads/feature/local
refs/remotes/origin/HEAD
refs/remotes/origin/main
refs/remotes/upstream/feature/remote
`)

	got := ParseBranchRefs(output)
	want := []Branch{
		{Name: "main"},
		{Name: "feature/local"},
		{Name: "main", Remote: "origin", RemoteOnly: true},
		{Name: "feature/remote", Remote: "upstream", RemoteOnly: true},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("expected branches %+v, got %+v", want, got)
	}
}

func TestService_DiscoverWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories and worktrees")
	}

	repoDir := initTempGitRepo(t)
	writeFile(t, filepath.Join(repoDir, "README.md"), "initial\n")
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "initial")

	featureDir := filepath.Join(t.TempDir(), "feature")
	runGit(t, repoDir, "worktree", "add", "-b", "feature", featureDir)
	writeFile(t, filepath.Join(featureDir, "dirty.txt"), "dirty\n")

	worktrees, err := (Service{}).DiscoverWorktrees(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("expected worktrees to be discovered, got %v", err)
	}

	mainWorktree := requireWorktree(t, worktrees, repoDir)
	if mainWorktree.Branch != "main" {
		t.Fatalf("expected main branch, got %+v", mainWorktree)
	}
	if mainWorktree.Dirty {
		t.Fatalf("expected main worktree to be clean, got %+v", mainWorktree)
	}

	featureWorktree := requireWorktree(t, worktrees, featureDir)
	if featureWorktree.Branch != "feature" {
		t.Fatalf("expected feature branch, got %+v", featureWorktree)
	}
	if !featureWorktree.Dirty || len(featureWorktree.StatusEntries) == 0 {
		t.Fatalf("expected feature worktree to be dirty, got %+v", featureWorktree)
	}
}

func TestService_DiscoverBranches(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repository branch refs")
	}

	repoDir := initTempGitRepo(t)
	writeFile(t, filepath.Join(repoDir, "README.md"), "initial\n")
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "initial")
	runGit(t, repoDir, "branch", "local-only")
	runGit(t, repoDir, "update-ref", "refs/remotes/origin/remote-only", "HEAD")

	branches, err := (Service{}).DiscoverBranches(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("expected branches to be discovered, got %v", err)
	}

	if !hasBranch(branches, Branch{Name: "local-only"}) {
		t.Fatalf("expected local-only branch in %+v", branches)
	}
	if !hasBranch(branches, Branch{Name: "remote-only", Remote: "origin", RemoteOnly: true}) {
		t.Fatalf("expected remote-only branch in %+v", branches)
	}
}

func TestService_DiscoverWorktreesMarksMissingWorktrees(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories and worktrees")
	}

	repoDir := initTempGitRepo(t)
	writeFile(t, filepath.Join(repoDir, "README.md"), "initial\n")
	runGit(t, repoDir, "add", "README.md")
	runGit(t, repoDir, "commit", "-m", "initial")

	missingDir := filepath.Join(t.TempDir(), "missing")
	runGit(t, repoDir, "worktree", "add", "-b", "missing", missingDir)
	if err := os.RemoveAll(missingDir); err != nil {
		t.Fatalf("remove missing worktree: %v", err)
	}

	worktrees, err := (Service{}).DiscoverWorktrees(context.Background(), repoDir)
	if err != nil {
		t.Fatalf("expected worktrees to be discovered, got %v", err)
	}
	missingWorktree := requireWorktree(t, worktrees, missingDir)
	if !missingWorktree.Missing {
		t.Fatalf("expected missing worktree flag, got %+v", missingWorktree)
	}
}

func requireWorktree(t *testing.T, worktrees []Worktree, path string) Worktree {
	t.Helper()
	wantPath := canonicalPath(t, path)
	for _, worktree := range worktrees {
		if canonicalPath(t, worktree.Path) == wantPath {
			return worktree
		}
	}
	t.Fatalf("worktree %q not found in %+v", path, worktrees)
	return Worktree{}
}

func hasBranch(branches []Branch, want Branch) bool {
	for _, branch := range branches {
		if branch == want {
			return true
		}
	}
	return false
}

func canonicalPath(t *testing.T, path string) string {
	t.Helper()
	canonical, err := filepath.EvalSymlinks(path)
	if err == nil {
		return canonical
	}
	parent, parentErr := filepath.EvalSymlinks(filepath.Dir(path))
	if parentErr == nil {
		return filepath.Join(parent, filepath.Base(path))
	}
	return filepath.Clean(path)
}

func initTempGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init", "-b", "main")
	runGit(t, dir, "config", "user.email", "test@example.com")
	runGit(t, dir, "config", "user.name", "Test User")
	return dir
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
