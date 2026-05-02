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

	var matchedPath string
	err = filepath.WalkDir(root, func(path string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			diagnostics = append(diagnostics, repositoryRootDiagnostic(rootIndex, fmt.Sprintf("read %q: %v", path, walkErr)))
			if entry != nil && entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if !entry.IsDir() {
			return nil
		}
		if entry.Name() == ".git" {
			return filepath.SkipDir
		}
		if !hasGitMetadata(path) {
			return nil
		}

		repo, err := CurrentGitRepository(path)
		if err != nil {
			diagnostics = append(diagnostics, Diagnostic{
				Path:    path,
				Message: err.Error(),
			})
			return nil
		}
		if sameRepoName(repo, repoName) {
			matchedRoot, err := CurrentGitRepositoryRoot(path)
			if err != nil {
				diagnostics = append(diagnostics, Diagnostic{
					Path:    path,
					Message: err.Error(),
				})
				return filepath.SkipDir
			}
			matchedPath = matchedRoot
			return fs.SkipAll
		}
		return nil
	})
	if err != nil {
		diagnostics = append(diagnostics, repositoryRootDiagnostic(rootIndex, fmt.Sprintf("scan %q: %v", root, err)))
	}
	return matchedPath, diagnostics
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
