package localrepo

import (
	"context"
	"fmt"
	"os"
	"os/exec"
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

// Runner executes Git commands for the local repository service.
type Runner interface {
	Run(ctx context.Context, dir string, args ...string) (string, error)
}

// GitRunner runs real Git commands.
type GitRunner struct{}

// Run executes git with -C dir and returns trimmed combined output.
func (GitRunner) Run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, strings.TrimSpace(string(output)))
	}
	return strings.TrimSpace(string(output)), nil
}

// Service discovers local repository state behind a Git command boundary.
type Service struct {
	Runner Runner
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

func porcelainStatusEntries(output string) []string {
	output = strings.TrimRight(output, "\r\n")
	if output == "" {
		return nil
	}
	return strings.Split(output, "\n")
}

func missingPath(path string) bool {
	_, err := os.Stat(path)
	return os.IsNotExist(err)
}
