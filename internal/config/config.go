package config

import (
	"fmt"
	"sort"
	"strings"
)

const (
	// StartupViewWorkbench is the initial repository workbench view.
	StartupViewWorkbench StartupView = "workbench"

	// PullRequestAny leaves pull request presence unfiltered.
	PullRequestAny PullRequestFilter = "any"
	// PullRequestPresent keeps work items that have a pull request.
	PullRequestPresent PullRequestFilter = "present"
	// PullRequestAbsent keeps work items that do not have a pull request.
	PullRequestAbsent PullRequestFilter = "absent"

	// LocalStatusAny leaves local status unfiltered.
	LocalStatusAny LocalStatusFilter = "any"
	// LocalStatusClean keeps clean local work items.
	LocalStatusClean LocalStatusFilter = "clean"
	// LocalStatusDirty keeps dirty local work items.
	LocalStatusDirty LocalStatusFilter = "dirty"
	// LocalStatusDetached keeps detached local work items.
	LocalStatusDetached LocalStatusFilter = "detached"
	// LocalStatusMissing keeps missing local work items.
	LocalStatusMissing LocalStatusFilter = "missing"
)

// StartupView identifies the view selected at startup.
type StartupView string

// PullRequestFilter configures workbench pull request presence filtering.
type PullRequestFilter string

// LocalStatusFilter configures workbench local status filtering.
type LocalStatusFilter string

// Config is a resolved runtime configuration.
type Config struct {
	Startup   StartupConfig   `toml:"startup"`
	UI        UIConfig        `toml:"ui"`
	Keys      KeyBindings     `toml:"keys"`
	Repos     ReposConfig     `toml:"repos"`
	Worktrees WorktreesConfig `toml:"worktrees"`
	Workbench WorkbenchConfig `toml:"workbench"`
}

// StartupConfig contains startup selection settings.
type StartupConfig struct {
	Repo string      `toml:"repo"`
	View StartupView `toml:"view"`
}

// UIConfig contains display preferences independent from rendering code.
type UIConfig struct {
	Theme        string  `toml:"theme"`
	PreviewWidth float64 `toml:"preview_width"`
	ShowIcons    bool    `toml:"show_icons"`
}

// KeyBindings maps action IDs to ordered key sequences.
type KeyBindings map[string][]string

// ReposConfig contains repository discovery and per-repository settings.
type ReposConfig struct {
	Roots        []string                    `toml:"roots"`
	Repositories map[string]RepositoryConfig `toml:"repositories"`
}

// RepositoryConfig contains settings scoped to one owner/repo.
type RepositoryConfig struct {
	DefaultBranch string `toml:"default_branch"`
	WorktreeRoot  string `toml:"worktree_root"`
}

// WorktreesConfig contains local worktree include and exclude patterns.
type WorktreesConfig struct {
	Include []string `toml:"include"`
	Exclude []string `toml:"exclude"`
}

// WorkbenchConfig contains filters used by the repository workbench.
type WorkbenchConfig struct {
	Filter         WorkbenchFilter            `toml:"filter"`
	BranchPatterns []string                   `toml:"branch_patterns"`
	SavedFilters   map[string]WorkbenchFilter `toml:"saved_filters"`
}

// WorkbenchFilter configures a deterministic workbench item filter.
type WorkbenchFilter struct {
	Worktree      string            `toml:"worktree"`
	BranchPattern string            `toml:"branch_pattern"`
	PullRequest   PullRequestFilter `toml:"pull_request"`
	LocalStatus   LocalStatusFilter `toml:"local_status"`
}

// PartialConfig is one optional configuration layer before merge resolution.
type PartialConfig struct {
	Startup     StartupConfigLayer   `toml:"startup"`
	UI          UIConfigLayer        `toml:"ui"`
	Keys        KeyBindings          `toml:"keys"`
	Repos       ReposConfigLayer     `toml:"repos"`
	Worktrees   WorktreesConfigLayer `toml:"worktrees"`
	Workbench   WorkbenchConfigLayer `toml:"workbench"`
	UnknownKeys []string             `toml:"-"`
}

// StartupConfigLayer is the optional form of StartupConfig.
type StartupConfigLayer struct {
	Repo *string      `toml:"repo"`
	View *StartupView `toml:"view"`
}

// UIConfigLayer is the optional form of UIConfig.
type UIConfigLayer struct {
	Theme        *string  `toml:"theme"`
	PreviewWidth *float64 `toml:"preview_width"`
	ShowIcons    *bool    `toml:"show_icons"`
}

// ReposConfigLayer is the optional form of ReposConfig.
type ReposConfigLayer struct {
	Roots        *[]string                        `toml:"roots"`
	Repositories map[string]RepositoryConfigLayer `toml:"repositories"`
}

// RepositoryConfigLayer is the optional form of RepositoryConfig.
type RepositoryConfigLayer struct {
	DefaultBranch *string `toml:"default_branch"`
	WorktreeRoot  *string `toml:"worktree_root"`
}

// WorktreesConfigLayer is the optional form of WorktreesConfig.
type WorktreesConfigLayer struct {
	Include *[]string `toml:"include"`
	Exclude *[]string `toml:"exclude"`
}

// WorkbenchConfigLayer is the optional form of WorkbenchConfig.
type WorkbenchConfigLayer struct {
	Filter         WorkbenchFilterLayer            `toml:"filter"`
	BranchPatterns *[]string                       `toml:"branch_patterns"`
	SavedFilters   map[string]WorkbenchFilterLayer `toml:"saved_filters"`
}

// WorkbenchFilterLayer is the optional form of WorkbenchFilter.
type WorkbenchFilterLayer struct {
	Worktree      *string            `toml:"worktree"`
	BranchPattern *string            `toml:"branch_pattern"`
	PullRequest   *PullRequestFilter `toml:"pull_request"`
	LocalStatus   *LocalStatusFilter `toml:"local_status"`
}

// Diagnostic describes a non-fatal configuration warning.
type Diagnostic struct {
	Path    string
	Message string
}

// Problem describes a fatal configuration validation problem.
type Problem struct {
	Path    string
	Message string
}

// ValidationError groups fatal configuration validation problems.
type ValidationError struct {
	Problems []Problem
}

func (e ValidationError) Error() string {
	if len(e.Problems) == 1 {
		problem := e.Problems[0]
		return fmt.Sprintf("%s: %s", problem.Path, problem.Message)
	}

	parts := make([]string, 0, len(e.Problems))
	for _, problem := range e.Problems {
		parts = append(parts, fmt.Sprintf("%s: %s", problem.Path, problem.Message))
	}
	return fmt.Sprintf("%d config validation errors: %s", len(e.Problems), strings.Join(parts, "; "))
}

// Defaults returns the built-in runtime configuration.
func Defaults() Config {
	return Config{
		Startup: StartupConfig{
			View: StartupViewWorkbench,
		},
		UI: UIConfig{
			Theme:        "default",
			PreviewWidth: 0.45,
			ShowIcons:    true,
		},
		Keys: KeyBindings{
			"move_down":           {"j", "down"},
			"move_up":             {"k", "up"},
			"jump_top":            {"g"},
			"jump_bottom":         {"G"},
			"focus_next_pane":     {"l", "tab"},
			"focus_previous_pane": {"h", "shift+tab"},
			"focus_pane_1":        {"1"},
			"focus_pane_2":        {"2"},
			"focus_pane_3":        {"3"},
			"toggle_help":         {"?"},
			"refresh":             {"r"},
			"open":                {"enter", "o"},
			"copy":                {"y"},
			"quit":                {"q", "esc", "ctrl+c"},
		},
		Repos: ReposConfig{
			Repositories: map[string]RepositoryConfig{},
		},
		Workbench: WorkbenchConfig{
			Filter:       defaultWorkbenchFilter(),
			SavedFilters: map[string]WorkbenchFilter{},
		},
	}
}

// MergeLayers applies all layers over the built-in defaults in order.
func MergeLayers(layers ...PartialConfig) Config {
	cfg := Defaults()
	for _, layer := range layers {
		cfg = Merge(cfg, layer)
	}
	return cfg
}

// Merge applies one stronger optional configuration layer over a resolved base.
func Merge(base Config, layer PartialConfig) Config {
	out := cloneConfig(base)

	if layer.Startup.Repo != nil {
		out.Startup.Repo = *layer.Startup.Repo
	}
	if layer.Startup.View != nil {
		out.Startup.View = *layer.Startup.View
	}

	if layer.UI.Theme != nil {
		out.UI.Theme = *layer.UI.Theme
	}
	if layer.UI.PreviewWidth != nil {
		out.UI.PreviewWidth = *layer.UI.PreviewWidth
	}
	if layer.UI.ShowIcons != nil {
		out.UI.ShowIcons = *layer.UI.ShowIcons
	}

	if len(layer.Keys) > 0 {
		if out.Keys == nil {
			out.Keys = KeyBindings{}
		}
		for action, bindings := range layer.Keys {
			out.Keys[action] = cloneStrings(bindings)
		}
	}

	if layer.Repos.Roots != nil {
		out.Repos.Roots = cloneStrings(*layer.Repos.Roots)
	}
	if len(layer.Repos.Repositories) > 0 {
		if out.Repos.Repositories == nil {
			out.Repos.Repositories = map[string]RepositoryConfig{}
		}
		for name, repoLayer := range layer.Repos.Repositories {
			repo := out.Repos.Repositories[name]
			if repoLayer.DefaultBranch != nil {
				repo.DefaultBranch = *repoLayer.DefaultBranch
			}
			if repoLayer.WorktreeRoot != nil {
				repo.WorktreeRoot = *repoLayer.WorktreeRoot
			}
			out.Repos.Repositories[name] = repo
		}
	}

	if layer.Worktrees.Include != nil {
		out.Worktrees.Include = cloneStrings(*layer.Worktrees.Include)
	}
	if layer.Worktrees.Exclude != nil {
		out.Worktrees.Exclude = cloneStrings(*layer.Worktrees.Exclude)
	}

	out.Workbench.Filter = mergeWorkbenchFilter(out.Workbench.Filter, layer.Workbench.Filter)
	if layer.Workbench.BranchPatterns != nil {
		out.Workbench.BranchPatterns = cloneStrings(*layer.Workbench.BranchPatterns)
	}
	if len(layer.Workbench.SavedFilters) > 0 {
		if out.Workbench.SavedFilters == nil {
			out.Workbench.SavedFilters = map[string]WorkbenchFilter{}
		}
		for name, filterLayer := range layer.Workbench.SavedFilters {
			filter, ok := out.Workbench.SavedFilters[name]
			if !ok {
				filter = defaultWorkbenchFilter()
			}
			out.Workbench.SavedFilters[name] = mergeWorkbenchFilter(filter, filterLayer)
		}
	}

	return out
}

// Validate returns an error when a resolved configuration contains invalid known values.
func Validate(cfg Config) error {
	var problems []Problem

	if cfg.Startup.View != StartupViewWorkbench {
		problems = append(problems, Problem{Path: "startup.view", Message: "must be workbench"})
	}
	if cfg.Startup.Repo != "" && !isRepoFullName(cfg.Startup.Repo) {
		problems = append(problems, Problem{Path: "startup.repo", Message: "must use owner/repo format"})
	}

	if !isSafeIdentifier(cfg.UI.Theme) {
		problems = append(problems, Problem{Path: "ui.theme", Message: "must be a safe identifier"})
	}
	if cfg.UI.PreviewWidth <= 0 || cfg.UI.PreviewWidth >= 1 {
		problems = append(problems, Problem{Path: "ui.preview_width", Message: "must be greater than 0 and less than 1"})
	}

	problems = append(problems, validateKeyBindings("keys", cfg.Keys)...)
	problems = append(problems, validateStringList("repos.roots", cfg.Repos.Roots)...)
	problems = append(problems, validateRepositories(cfg.Repos.Repositories)...)
	problems = append(problems, validateStringList("worktrees.include", cfg.Worktrees.Include)...)
	problems = append(problems, validateStringList("worktrees.exclude", cfg.Worktrees.Exclude)...)
	problems = append(problems, validateStringList("workbench.branch_patterns", cfg.Workbench.BranchPatterns)...)
	problems = append(problems, validateWorkbenchFilter("workbench.filter", cfg.Workbench.Filter)...)
	problems = append(problems, validateSavedFilters(cfg.Workbench.SavedFilters)...)

	if len(problems) > 0 {
		return ValidationError{Problems: problems}
	}
	return nil
}

// ValidateLayer validates a single optional layer and returns non-fatal unknown-key diagnostics.
func ValidateLayer(layer PartialConfig) ([]Diagnostic, error) {
	warnings := make([]Diagnostic, 0, len(layer.UnknownKeys))
	for _, key := range layer.UnknownKeys {
		path := strings.TrimSpace(key)
		if path == "" {
			path = "<unknown>"
		}
		warnings = append(warnings, Diagnostic{Path: path, Message: "unknown configuration key"})
	}

	if err := Validate(Merge(Defaults(), layer)); err != nil {
		return warnings, err
	}
	return warnings, nil
}

func mergeWorkbenchFilter(base WorkbenchFilter, layer WorkbenchFilterLayer) WorkbenchFilter {
	if base.PullRequest == "" {
		base.PullRequest = PullRequestAny
	}
	if base.LocalStatus == "" {
		base.LocalStatus = LocalStatusAny
	}
	if layer.Worktree != nil {
		base.Worktree = *layer.Worktree
	}
	if layer.BranchPattern != nil {
		base.BranchPattern = *layer.BranchPattern
	}
	if layer.PullRequest != nil {
		base.PullRequest = *layer.PullRequest
	}
	if layer.LocalStatus != nil {
		base.LocalStatus = *layer.LocalStatus
	}
	return base
}

func defaultWorkbenchFilter() WorkbenchFilter {
	return WorkbenchFilter{
		PullRequest: PullRequestAny,
		LocalStatus: LocalStatusAny,
	}
}

func cloneConfig(cfg Config) Config {
	return Config{
		Startup: cfg.Startup,
		UI:      cfg.UI,
		Keys:    cloneKeyBindings(cfg.Keys),
		Repos: ReposConfig{
			Roots:        cloneStrings(cfg.Repos.Roots),
			Repositories: cloneRepositories(cfg.Repos.Repositories),
		},
		Worktrees: WorktreesConfig{
			Include: cloneStrings(cfg.Worktrees.Include),
			Exclude: cloneStrings(cfg.Worktrees.Exclude),
		},
		Workbench: WorkbenchConfig{
			Filter:         cfg.Workbench.Filter,
			BranchPatterns: cloneStrings(cfg.Workbench.BranchPatterns),
			SavedFilters:   cloneSavedFilters(cfg.Workbench.SavedFilters),
		},
	}
}

func cloneKeyBindings(bindings KeyBindings) KeyBindings {
	if bindings == nil {
		return nil
	}
	out := make(KeyBindings, len(bindings))
	for action, keys := range bindings {
		out[action] = cloneStrings(keys)
	}
	return out
}

func cloneRepositories(repos map[string]RepositoryConfig) map[string]RepositoryConfig {
	if repos == nil {
		return nil
	}
	out := make(map[string]RepositoryConfig, len(repos))
	for name, repo := range repos {
		out[name] = repo
	}
	return out
}

func cloneSavedFilters(filters map[string]WorkbenchFilter) map[string]WorkbenchFilter {
	if filters == nil {
		return nil
	}
	out := make(map[string]WorkbenchFilter, len(filters))
	for name, filter := range filters {
		out[name] = filter
	}
	return out
}

func cloneStrings(values []string) []string {
	if values == nil {
		return nil
	}
	return append([]string(nil), values...)
}

func validateKeyBindings(path string, bindings KeyBindings) []Problem {
	var problems []Problem
	if len(bindings) == 0 {
		return append(problems, Problem{Path: path, Message: "must define at least one action"})
	}

	actions := make([]string, 0, len(bindings))
	for action := range bindings {
		actions = append(actions, action)
	}
	sort.Strings(actions)

	for _, action := range actions {
		actionPath := path + "." + action
		if !isSafeIdentifier(action) {
			problems = append(problems, Problem{Path: actionPath, Message: "action must be a safe identifier"})
		}
		keys := bindings[action]
		if len(keys) == 0 {
			problems = append(problems, Problem{Path: actionPath, Message: "must define at least one key"})
			continue
		}
		for i, key := range keys {
			if strings.TrimSpace(key) == "" {
				problems = append(problems, Problem{Path: fmt.Sprintf("%s[%d]", actionPath, i), Message: "key must not be empty"})
			}
		}
	}

	return problems
}

func validateStringList(path string, values []string) []Problem {
	var problems []Problem
	for i, value := range values {
		if strings.TrimSpace(value) == "" {
			problems = append(problems, Problem{Path: fmt.Sprintf("%s[%d]", path, i), Message: "must not be empty"})
		}
	}
	return problems
}

func validateRepositories(repos map[string]RepositoryConfig) []Problem {
	var problems []Problem
	names := make([]string, 0, len(repos))
	for name := range repos {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if !isRepoFullName(name) {
			problems = append(problems, Problem{Path: fmt.Sprintf("repos.repositories.%s", name), Message: "must use owner/repo format"})
		}
		if strings.TrimSpace(repos[name].DefaultBranch) != repos[name].DefaultBranch {
			problems = append(problems, Problem{Path: fmt.Sprintf("repos.repositories.%s.default_branch", name), Message: "must not have surrounding whitespace"})
		}
		if strings.TrimSpace(repos[name].WorktreeRoot) != repos[name].WorktreeRoot {
			problems = append(problems, Problem{Path: fmt.Sprintf("repos.repositories.%s.worktree_root", name), Message: "must not have surrounding whitespace"})
		}
	}
	return problems
}

func validateSavedFilters(filters map[string]WorkbenchFilter) []Problem {
	var problems []Problem
	names := make([]string, 0, len(filters))
	for name := range filters {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		if !isSafeIdentifier(name) {
			problems = append(problems, Problem{Path: fmt.Sprintf("workbench.saved_filters.%s", name), Message: "name must be a safe identifier"})
		}
		problems = append(problems, validateWorkbenchFilter(fmt.Sprintf("workbench.saved_filters.%s", name), filters[name])...)
	}
	return problems
}

func validateWorkbenchFilter(path string, filter WorkbenchFilter) []Problem {
	var problems []Problem
	if strings.TrimSpace(filter.Worktree) != filter.Worktree {
		problems = append(problems, Problem{Path: path + ".worktree", Message: "must not have surrounding whitespace"})
	}
	if strings.TrimSpace(filter.BranchPattern) != filter.BranchPattern {
		problems = append(problems, Problem{Path: path + ".branch_pattern", Message: "must not have surrounding whitespace"})
	}
	if !isPullRequestFilter(filter.PullRequest) {
		problems = append(problems, Problem{Path: path + ".pull_request", Message: "must be any, present, or absent"})
	}
	if !isLocalStatusFilter(filter.LocalStatus) {
		problems = append(problems, Problem{Path: path + ".local_status", Message: "must be any, clean, dirty, detached, or missing"})
	}
	return problems
}

func isPullRequestFilter(value PullRequestFilter) bool {
	switch value {
	case PullRequestAny, PullRequestPresent, PullRequestAbsent:
		return true
	default:
		return false
	}
}

func isLocalStatusFilter(value LocalStatusFilter) bool {
	switch value {
	case LocalStatusAny, LocalStatusClean, LocalStatusDirty, LocalStatusDetached, LocalStatusMissing:
		return true
	default:
		return false
	}
}

func isRepoFullName(value string) bool {
	owner, name, ok := strings.Cut(value, "/")
	if !ok || owner == "" || name == "" || strings.Contains(name, "/") {
		return false
	}
	return isRepoOwner(owner) && isRepoName(name)
}

func isRepoOwner(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-':
		default:
			return false
		}
	}
	return true
}

func isRepoName(value string) bool {
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '-' || r == '_' || r == '.':
		default:
			return false
		}
	}
	return true
}

func isSafeIdentifier(value string) bool {
	if value == "" {
		return false
	}
	for _, r := range value {
		switch {
		case r >= 'a' && r <= 'z':
		case r >= 'A' && r <= 'Z':
		case r >= '0' && r <= '9':
		case r == '_' || r == '-':
		default:
			return false
		}
	}
	return true
}
