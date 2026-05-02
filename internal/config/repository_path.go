package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// RepositoryPathOptions configures local checkout path resolution.
type RepositoryPathOptions struct {
	Repo       string
	Config     Config
	WorkingDir string
}

// RepositoryPathResult contains a resolved local path and non-fatal diagnostics.
type RepositoryPathResult struct {
	Repo        string
	Path        string
	Diagnostics []Diagnostic
}

// ResolveRepositoryPath maps an owner/repo name to a local checkout path.
func ResolveRepositoryPath(options RepositoryPathOptions) RepositoryPathResult {
	result := RepositoryPathResult{Repo: options.Repo}
	if options.Repo == "" {
		return result
	}

	currentRepo, currentErr := CurrentGitRepository(options.WorkingDir)
	if currentErr == nil {
		if sameRepoName(currentRepo, options.Repo) {
			if root, err := CurrentGitRepositoryRoot(options.WorkingDir); err == nil {
				result.Path = root
				return result
			}
		}
	} else {
		result.Diagnostics = append(result.Diagnostics, Diagnostic{
			Path:    "current_git",
			Message: currentErr.Error(),
		})
	}

	for i, root := range options.Config.Repos.Roots {
		rootPath := expandHomePath(root)
		matchedPath, diagnostics := findRepositoryInRoot(rootPath, options.Repo, i)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
		if matchedPath != "" {
			result.Path = matchedPath
			return result
		}
	}

	result.Diagnostics = append(result.Diagnostics, Diagnostic{
		Path:    "repos",
		Message: fmt.Sprintf("no local checkout found for %s", options.Repo),
	})
	return result
}

func findRepositoryInRoot(root string, repoName string, rootIndex int) (string, []Diagnostic) {
	diagnostics := []Diagnostic{}
	info, err := os.Stat(root)
	if err != nil {
		return "", []Diagnostic{repositoryRootDiagnostic(rootIndex, fmt.Sprintf("%q is not accessible: %v", root, err))}
	}
	if !info.IsDir() {
		return "", []Diagnostic{repositoryRootDiagnostic(rootIndex, fmt.Sprintf("%q is not a directory", root))}
	}
	walkRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", []Diagnostic{repositoryRootDiagnostic(rootIndex, fmt.Sprintf("resolve %q: %v", root, err))}
	}

	visited := map[string]struct{}{walkRoot: {}}
	var matchedPath string
	err = walkRepositoryRoot(root, walkRoot, repoName, rootIndex, visited, &matchedPath, &diagnostics)
	if err != nil {
		diagnostics = append(diagnostics, repositoryRootDiagnostic(rootIndex, fmt.Sprintf("scan %q: %v", root, err)))
	}
	return matchedPath, diagnostics
}

func walkRepositoryRoot(root string, walkRoot string, repoName string, rootIndex int, visited map[string]struct{}, matchedPath *string, diagnostics *[]Diagnostic) error {
	return filepath.WalkDir(walkRoot, func(path string, entry fs.DirEntry, walkErr error) error {
		diagnosticPath := repositoryWalkDiagnosticPath(root, walkRoot, path)
		if walkErr != nil {
			*diagnostics = append(*diagnostics, repositoryRootDiagnostic(rootIndex, fmt.Sprintf("read %q: %v", diagnosticPath, walkErr)))
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
			if _, seen := visited[linkedRoot]; seen {
				return nil
			}
			visited[linkedRoot] = struct{}{}
			if err := walkRepositoryRoot(diagnosticPath, linkedRoot, repoName, rootIndex, visited, matchedPath, diagnostics); err != nil {
				*diagnostics = append(*diagnostics, repositoryRootDiagnostic(rootIndex, fmt.Sprintf("scan %q: %v", diagnosticPath, err)))
			}
			if *matchedPath != "" {
				return fs.SkipAll
			}
			return nil
		}
		if !hasGitMetadata(path) {
			return nil
		}

		repo, err := CurrentGitRepository(path)
		if err != nil {
			*diagnostics = append(*diagnostics, Diagnostic{
				Path:    diagnosticPath,
				Message: err.Error(),
			})
			return nil
		}
		if sameRepoName(repo, repoName) {
			matchedRoot, err := CurrentGitRepositoryRoot(path)
			if err != nil {
				*diagnostics = append(*diagnostics, Diagnostic{
					Path:    diagnosticPath,
					Message: err.Error(),
				})
				return filepath.SkipDir
			}
			*matchedPath = matchedRoot
			return fs.SkipAll
		}
		return nil
	})
}

func repositorySymlinkDir(path string, diagnosticPath string, diagnostics *[]Diagnostic) (string, bool) {
	info, err := os.Stat(path)
	if err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{
			Path:    diagnosticPath,
			Message: err.Error(),
		})
		return "", false
	}
	if !info.IsDir() {
		return "", false
	}
	linkedRoot, err := filepath.EvalSymlinks(path)
	if err != nil {
		*diagnostics = append(*diagnostics, Diagnostic{
			Path:    diagnosticPath,
			Message: fmt.Sprintf("resolve symlink: %v", err),
		})
		return "", false
	}
	return linkedRoot, true
}

func repositoryWalkDiagnosticPath(root string, walkRoot string, path string) string {
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

func hasGitMetadata(path string) bool {
	_, err := os.Stat(filepath.Join(path, ".git"))
	return err == nil
}

func repositoryRootDiagnostic(index int, message string) Diagnostic {
	return Diagnostic{
		Path:    fmt.Sprintf("repos.roots[%d]", index),
		Message: message,
	}
}

func sameRepoName(left string, right string) bool {
	return strings.EqualFold(left, right)
}

func expandHomePath(path string) string {
	if path == "~" {
		if home, err := os.UserHomeDir(); err == nil {
			return home
		}
	}
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, filepath.FromSlash(strings.TrimPrefix(path, "~/")))
		}
	}
	return path
}
