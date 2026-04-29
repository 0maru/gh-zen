package workbench

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/0maru/gh-zen/internal/localrepo"
)

type fakeLocalDiscovery struct {
	worktrees []localrepo.Worktree
	branches  []localrepo.Branch
	err       error
}

func (f fakeLocalDiscovery) DiscoverWorktrees(context.Context, string) ([]localrepo.Worktree, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.worktrees, nil
}

func (f fakeLocalDiscovery) DiscoverBranches(context.Context, string) ([]localrepo.Branch, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.branches, nil
}

func TestAssembleLocalWorkItems_CoversWorktreeLocalAndRemoteBranches(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := AssembleLocalWorkItems(
		repo,
		[]localrepo.Worktree{
			{Path: "/repo", Branch: "main"},
			{Path: "/repo-feature", Branch: "feature", Dirty: true, StatusEntries: []string{" M file.go", "?? new.go"}},
		},
		[]localrepo.Branch{
			{Name: "feature"},
			{Name: "local-only"},
			{Name: "remote-only", Remote: "origin", RemoteOnly: true},
			{Name: "main", Remote: "origin", RemoteOnly: true},
		},
	)

	if len(items) != 4 {
		t.Fatalf("expected worktree, local-only, and remote-only items, got %+v", items)
	}
	if items[0].Worktree == nil || items[0].Branch.Name != "main" || items[0].Local.State != LocalClean {
		t.Fatalf("expected clean main worktree item, got %+v", items[0])
	}
	if items[1].Worktree == nil || items[1].Branch.Name != "feature" || items[1].Local.State != LocalDirty {
		t.Fatalf("expected dirty feature worktree item, got %+v", items[1])
	}
	if items[2].Worktree != nil || items[2].Branch.Name != "local-only" || items[2].Branch.RemoteOnly {
		t.Fatalf("expected local branch placeholder, got %+v", items[2])
	}
	if items[3].Worktree != nil || items[3].Branch.Name != "remote-only" || !items[3].Branch.RemoteOnly {
		t.Fatalf("expected remote-only placeholder, got %+v", items[3])
	}
	if items[3].Local.State != LocalMissing {
		t.Fatalf("expected remote-only local state to be missing, got %+v", items[3].Local)
	}
}

func TestAssembleLocalWorkItems_RepresentsDetachedAndMissingWorktrees(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := AssembleLocalWorkItems(repo, []localrepo.Worktree{
		{Path: "/repo-detached", Detached: true},
		{Path: "/repo-missing", Branch: "missing", Missing: true, Prunable: true},
	}, nil)

	gotStates := []LocalState{items[0].Local.State, items[1].Local.State}
	wantStates := []LocalState{LocalDetached, LocalMissing}
	if !reflect.DeepEqual(gotStates, wantStates) {
		t.Fatalf("expected states %#v, got %#v", wantStates, gotStates)
	}
}

func TestLocalWorkItemService_ReturnsErrorItem(t *testing.T) {
	repo := RepoRef{Owner: "0maru", Name: "gh-zen"}
	items := (LocalWorkItemService{
		Repo:      repo,
		RepoPath:  "/repo",
		Discovery: fakeLocalDiscovery{err: errors.New("fake discovery failed")},
	}).WorkItems(context.Background())

	if len(items) != 1 {
		t.Fatalf("expected one error item, got %+v", items)
	}
	if items[0].Title() != "local discovery error" {
		t.Fatalf("expected error title, got %q", items[0].Title())
	}
	if items[0].Local == nil || !strings.Contains(items[0].Local.Summary, "fake discovery failed") {
		t.Fatalf("expected error summary, got %+v", items[0].Local)
	}
}

func TestLocalWorkItemService_WithTemporaryGitRepository(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary Git repositories")
	}

	repoDir := initLocalServiceRepo(t)
	featureDir := filepath.Join(t.TempDir(), "feature")
	runLocalServiceGit(t, repoDir, "worktree", "add", "-b", "feature", featureDir)
	writeLocalServiceFile(t, filepath.Join(featureDir, "dirty.txt"), "dirty\n")
	runLocalServiceGit(t, repoDir, "branch", "local-only")
	runLocalServiceGit(t, repoDir, "update-ref", "refs/remotes/origin/remote-only", "HEAD")

	items := (LocalWorkItemService{
		Repo:      RepoRef{Owner: "0maru", Name: "gh-zen"},
		RepoPath:  repoDir,
		Discovery: localrepo.Service{},
	}).WorkItems(context.Background())

	if !hasWorkItem(items, func(item WorkItem) bool {
		return item.Worktree != nil && item.Branch != nil && item.Branch.Name == "feature" && item.Local.State == LocalDirty
	}) {
		t.Fatalf("expected dirty feature worktree item, got %+v", items)
	}
	if !hasWorkItem(items, func(item WorkItem) bool {
		return item.Worktree == nil && item.Branch != nil && item.Branch.Name == "local-only" && !item.Branch.RemoteOnly
	}) {
		t.Fatalf("expected local-only branch placeholder, got %+v", items)
	}
	if !hasWorkItem(items, func(item WorkItem) bool {
		return item.Worktree == nil && item.Branch != nil && item.Branch.Name == "remote-only" && item.Branch.RemoteOnly
	}) {
		t.Fatalf("expected remote-only branch placeholder, got %+v", items)
	}
}

func hasWorkItem(items []WorkItem, match func(WorkItem) bool) bool {
	for _, item := range items {
		if match(item) {
			return true
		}
	}
	return false
}

func initLocalServiceRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runLocalServiceGit(t, dir, "init", "-b", "main")
	runLocalServiceGit(t, dir, "config", "user.email", "test@example.com")
	runLocalServiceGit(t, dir, "config", "user.name", "Test User")
	writeLocalServiceFile(t, filepath.Join(dir, "README.md"), "initial\n")
	runLocalServiceGit(t, dir, "add", "README.md")
	runLocalServiceGit(t, dir, "commit", "-m", "initial")
	return dir
}

func runLocalServiceGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func writeLocalServiceFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
