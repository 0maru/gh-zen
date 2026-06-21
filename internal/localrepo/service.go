package localrepo

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

// Worktree describes one local Git worktree discovered from porcelain output.
type Worktree struct {
	Path           string
	Head           string
	Branch         string
	Detached       bool
	Missing        bool
	Prunable       bool
	PrunableReason string
	Dirty          bool
	StatusEntries  []string
}

// Branch describes one local or remote branch ref discoverable in Git.
type Branch struct {
	Name       string
	Remote     string
	RemoteOnly bool
}

// Repository describes one local Git checkout discovered below configured roots.
type Repository struct {
	Path          string
	OriginURL     string
	DefaultBranch string
	Remotes       []string
}

// RepositoryDiagnostic describes a non-fatal repository discovery warning.
type RepositoryDiagnostic struct {
	Path    string
	Message string
}

// Runner executes Git commands for the local repository service.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// GitRunner runs real Git commands.
type GitRunner struct{}

// Run executes git with -C dir and returns combined output without trailing newlines.
func (GitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return trimGitOutput(output), nil
}

// Service discovers local repository state behind a Git command boundary.
type Service struct {
	Runner Runner
}

// DiscoverRepositories finds Git repositories below the configured roots.
func (s Service) DiscoverRepositories(ctx context.Context, roots []string) ([]Repository, []RepositoryDiagnostic) {
	repositories := []Repository{}
	diagnostics := []RepositoryDiagnostic{}
	seen := map[string]struct{}{}
	visited := map[string]struct{}{}
	for _, root := range roots {
		walkRoot, err := filepath.EvalSymlinks(root)
		if err != nil {
			diagnostics = append(diagnostics, RepositoryDiagnostic{Path: root, Message: fmt.Sprintf("resolve root: %v", err)})
			continue
		}
		if _, ok := visited[walkRoot]; ok {
			continue
		}
		visited[walkRoot] = struct{}{}
		s.discoverRepositoriesInRoot(ctx, root, walkRoot, visited, seen, &repositories, &diagnostics)
	}
	sort.SliceStable(repositories, func(i, j int) bool {
		if repositories[i].Path == repositories[j].Path {
			return repositories[i].OriginURL < repositories[j].OriginURL
		}
		return repositories[i].Path < repositories[j].Path
	})
	return repositories, diagnostics
}

func (s Service) discoverRepositoriesInRoot(ctx context.Context, displayRoot string, walkRoot string, visited map[string]struct{}, seen map[string]struct{}, repositories *[]Repository, diagnostics *[]RepositoryDiagnostic) {
	err := filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		diagnosticPath := repositoryDiagnosticPath(displayRoot, walkRoot, path)
		if walkErr != nil {
			*diagnostics = append(*diagnostics, RepositoryDiagnostic{Path: diagnosticPath, Message: fmt.Sprintf("read: %v", walkErr)})
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Name() == ".git" {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			if entry.Type()&fs.ModeSymlink == 0 {
				return nil
			}
			linkedRoot, ok := repositorySymlinkDir(path, diagnosticPath, diagnostics)
			if !ok {
				return nil
			}
			if _, ok := visited[linkedRoot]; ok {
				return nil
			}
			visited[linkedRoot] = struct{}{}
			s.discoverRepositoriesInRoot(ctx, diagnosticPath, linkedRoot, visited, seen, repositories, diagnostics)
			return nil
		}
		if !hasGitMetadata(path) {
			return nil
		}
		repository, err := s.RepositoryMetadata(ctx, path)
		if err != nil {
			*diagnostics = append(*diagnostics, RepositoryDiagnostic{Path: diagnosticPath, Message: fmt.Sprintf("read repository metadata: %v", err)})
			return nil
		}
		if repository.Path == "" {
			repository.Path = path
		}
		if _, ok := seen[repository.Path]; ok {
			return nil
		}
		seen[repository.Path] = struct{}{}
		*repositories = append(*repositories, repository)
		return nil
	})
	if err != nil {
		*diagnostics = append(*diagnostics, RepositoryDiagnostic{Path: displayRoot, Message: fmt.Sprintf("scan: %v", err)})
	}
}

// RepositoryMetadata reads repository-level metadata through Git.
func (s Service) RepositoryMetadata(ctx context.Context, repoPath string) (Repository, error) {
	runner := s.runner()
	root, err := runner.Run(ctx, repoPath, "rev-parse", "--show-toplevel")
	if err != nil {
		return Repository{}, fmt.Errorf("resolve repository root: %w", err)
	}
	originURL, err := runner.Run(ctx, root, "remote", "get-url", "origin")
	if err != nil {
		return Repository{}, fmt.Errorf("read origin remote: %w", err)
	}
	remoteOutput, err := runner.Run(ctx, root, "remote")
	if err != nil {
		return Repository{}, fmt.Errorf("list remotes: %w", err)
	}
	return Repository{
		Path:          root,
		OriginURL:     originURL,
		DefaultBranch: s.defaultBranch(ctx, root),
		Remotes:       parseRemoteNames(remoteOutput),
	}, nil
}

func (s Service) defaultBranch(ctx context.Context, repoPath string) string {
	runner := s.runner()
	branch, err := runner.Run(ctx, repoPath, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD")
	if err == nil && branch != "" {
		return strings.TrimPrefix(branch, "origin/")
	}
	branch, err = runner.Run(ctx, repoPath, "symbolic-ref", "--quiet", "--short", "HEAD")
	if err == nil {
		return branch
	}
	return ""
}

// DiscoverWorktrees lists local worktrees and reads their dirty status.
func (s Service) DiscoverWorktrees(ctx context.Context, repoPath string) ([]Worktree, error) {
	runner := s.runner()
	output, err := runner.Run(ctx, repoPath, "worktree", "list", "--porcelain")
	if err != nil {
		return nil, fmt.Errorf("list worktrees: %w", err)
	}

	worktrees, err := ParseWorktreeListPorcelain(output)
	if err != nil {
		return nil, err
	}
	for i := range worktrees {
		worktree := &worktrees[i]
		if worktree.Prunable {
			if missingPath(worktree.Path) {
				worktree.Missing = true
			}
			continue
		}
		if missingPath(worktree.Path) {
			worktree.Missing = true
			continue
		}
		status, err := runner.Run(ctx, worktree.Path, "status", "--porcelain=v1")
		if err != nil {
			return nil, fmt.Errorf("read status for %q: %w", worktree.Path, err)
		}
		worktree.StatusEntries = porcelainStatusEntries(status)
		worktree.Dirty = len(worktree.StatusEntries) > 0
	}
	return worktrees, nil
}

// DiscoverBranches lists local and remote branch refs known to the repository.
func (s Service) DiscoverBranches(ctx context.Context, repoPath string) ([]Branch, error) {
	runner := s.runner()
	output, err := runner.Run(ctx, repoPath, "for-each-ref", "--format=%(refname)", "refs/heads", "refs/remotes")
	if err != nil {
		return nil, fmt.Errorf("list branches: %w", err)
	}
	remoteOutput, err := runner.Run(ctx, repoPath, "remote")
	if err != nil {
		return nil, fmt.Errorf("list remotes: %w", err)
	}
	return ParseBranchRefsWithRemotes(output, parseRemoteNames(remoteOutput)), nil
}

func (s Service) runner() Runner {
	if s.Runner != nil {
		return s.Runner
	}
	return GitRunner{}
}

// ParseWorktreeListPorcelain parses git worktree list --porcelain output.
func ParseWorktreeListPorcelain(output string) ([]Worktree, error) {
	blocks := strings.Split(strings.TrimSpace(output), "\n\n")
	if len(blocks) == 1 && strings.TrimSpace(blocks[0]) == "" {
		return nil, nil
	}

	worktrees := make([]Worktree, 0, len(blocks))
	for _, block := range blocks {
		var worktree Worktree
		for _, line := range strings.Split(block, "\n") {
			if line == "" {
				continue
			}
			key, value, hasValue := strings.Cut(line, " ")
			switch key {
			case "worktree":
				if !hasValue || value == "" {
					return nil, fmt.Errorf("worktree porcelain entry missing path")
				}
				worktree.Path = value
			case "HEAD":
				worktree.Head = value
			case "branch":
				worktree.Branch = strings.TrimPrefix(value, "refs/heads/")
			case "detached":
				worktree.Detached = true
			case "prunable":
				worktree.Prunable = true
				worktree.PrunableReason = value
			}
		}
		if worktree.Path == "" {
			return nil, fmt.Errorf("worktree porcelain block missing worktree path")
		}
		worktrees = append(worktrees, worktree)
	}
	return worktrees, nil
}

// ParseBranchRefs parses refnames from git for-each-ref --format=%(refname).
func ParseBranchRefs(output string) []Branch {
	return ParseBranchRefsWithRemotes(output, nil)
}

// ParseBranchRefsWithRemotes parses refnames using known remote names for disambiguation.
func ParseBranchRefsWithRemotes(output string, remotes []string) []Branch {
	branches := []Branch{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		ref := strings.TrimSpace(line)
		switch {
		case ref == "":
			continue
		case strings.HasPrefix(ref, "refs/heads/"):
			branches = append(branches, Branch{Name: strings.TrimPrefix(ref, "refs/heads/")})
		case strings.HasPrefix(ref, "refs/remotes/"):
			remoteRef := strings.TrimPrefix(ref, "refs/remotes/")
			remote, name, ok := splitRemoteBranch(remoteRef, remotes)
			if !ok || name == "" || name == "HEAD" {
				continue
			}
			branches = append(branches, Branch{Name: name, Remote: remote, RemoteOnly: true})
		}
	}
	return branches
}

func parseRemoteNames(output string) []string {
	remotes := []string{}
	for _, line := range strings.Split(strings.TrimSpace(output), "\n") {
		remote := strings.TrimSpace(line)
		if remote != "" {
			remotes = append(remotes, remote)
		}
	}
	return remotes
}

func splitRemoteBranch(remoteRef string, remotes []string) (string, string, bool) {
	for _, remote := range remotesByLength(remotes) {
		prefix := remote + "/"
		if strings.HasPrefix(remoteRef, prefix) {
			return remote, strings.TrimPrefix(remoteRef, prefix), true
		}
	}
	return strings.Cut(remoteRef, "/")
}

func remotesByLength(remotes []string) []string {
	out := append([]string(nil), remotes...)
	sort.SliceStable(out, func(i, j int) bool {
		return len(out[i]) > len(out[j])
	})
	return out
}

func porcelainStatusEntries(output string) []string {
	output = strings.TrimRight(output, "\r\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

func trimGitOutput(output []byte) string {
	return strings.TrimRight(string(output), "\r\n")
}

func missingPath(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}

func repositoryDiagnosticPath(root string, walkRoot string, path string) string {
	if root == walkRoot {
		return path
	}
	rel, err := filepath.Rel(walkRoot, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	if rel == "." {
		return root
	}
	return filepath.Join(root, rel)
}

func repositorySymlinkDir(path string, diagnosticPath string, diagnostics *[]RepositoryDiagnostic) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		*diagnostics = append(*diagnostics, RepositoryDiagnostic{Path: diagnosticPath, Message: err.Error()})
		return "", false
	}
	if !info.IsDir() {
		return "", false
	}
	linkedRoot, err := filepath.EvalSymlinks(path)
	if err != nil {
		*diagnostics = append(*diagnostics, RepositoryDiagnostic{Path: diagnosticPath, Message: fmt.Sprintf("resolve symlink: %v", err)})
		return "", false
	}
	return linkedRoot, true
}

func hasGitMetadata(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}
