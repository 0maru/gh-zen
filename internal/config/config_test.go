package config

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestDefaults_AreUsable(t *testing.T) {
	got := Defaults()

	if err := Validate(got); err != nil {
		t.Fatalf("expected defaults to validate, got %v", err)
	}
	if got.Startup.View != StartupViewWorkbench {
		t.Fatalf("expected default startup view %q, got %q", StartupViewWorkbench, got.Startup.View)
	}
	if got.UI.Theme != "default" || got.UI.PreviewWidth != 0.45 || !got.UI.ShowIcons {
		t.Fatalf("unexpected default UI config: %+v", got.UI)
	}
	if len(got.Keys["quit"]) == 0 {
		t.Fatalf("expected default quit key bindings")
	}
	if got.Workbench.Filter.PullRequest != PullRequestAny || got.Workbench.Filter.LocalStatus != LocalStatusAny {
		t.Fatalf("expected unfiltered workbench default, got %+v", got.Workbench.Filter)
	}
}

func TestMergeLayers_ScalarsUseLastWriterWins(t *testing.T) {
	globalRepo := "0maru/gh-zen"
	projectRepo := "0maru/dotfiles"
	theme := "high_contrast"
	previewWidth := 0.35
	showIcons := false

	got := MergeLayers(
		PartialConfig{
			Startup: StartupConfigLayer{Repo: str(globalRepo)},
			UI:      UIConfigLayer{Theme: str(theme)},
		},
		PartialConfig{
			Startup: StartupConfigLayer{Repo: str(projectRepo)},
			UI: UIConfigLayer{
				PreviewWidth: float64Ptr(previewWidth),
				ShowIcons:    boolPtr(showIcons),
			},
		},
	)

	if got.Startup.Repo != projectRepo {
		t.Fatalf("expected stronger startup repo %q, got %q", projectRepo, got.Startup.Repo)
	}
	if got.UI.Theme != theme {
		t.Fatalf("expected weaker theme to remain %q, got %q", theme, got.UI.Theme)
	}
	if got.UI.PreviewWidth != previewWidth {
		t.Fatalf("expected preview width %.2f, got %.2f", previewWidth, got.UI.PreviewWidth)
	}
	if got.UI.ShowIcons {
		t.Fatalf("expected false scalar override to be preserved")
	}
}

func TestMerge_KeyBindingsReplacePerAction(t *testing.T) {
	got := Merge(Defaults(), PartialConfig{
		Keys: KeyBindings{
			"open": {"x"},
		},
	})

	if !reflect.DeepEqual(got.Keys["open"], []string{"x"}) {
		t.Fatalf("expected open binding to be replaced, got %#v", got.Keys["open"])
	}
	if !reflect.DeepEqual(got.Keys["quit"], []string{"q", "esc", "ctrl+c"}) {
		t.Fatalf("expected unrelated key binding to remain, got %#v", got.Keys["quit"])
	}
}

func TestMerge_ListsReplace(t *testing.T) {
	got := MergeLayers(
		PartialConfig{
			Repos:     ReposConfigLayer{Roots: stringList("~/src", "~/work")},
			Worktrees: WorktreesConfigLayer{Include: stringList("~/src/*")},
			Workbench: WorkbenchConfigLayer{
				BranchPatterns: stringList("feat/*", "fix/*"),
			},
		},
		PartialConfig{
			Repos: ReposConfigLayer{Roots: stringList("~/repos")},
			Workbench: WorkbenchConfigLayer{
				BranchPatterns: stringList(),
			},
		},
	)

	if !reflect.DeepEqual(got.Repos.Roots, []string{"~/repos"}) {
		t.Fatalf("expected repo roots to be replaced, got %#v", got.Repos.Roots)
	}
	if !reflect.DeepEqual(got.Worktrees.Include, []string{"~/src/*"}) {
		t.Fatalf("expected untouched worktree include list to remain, got %#v", got.Worktrees.Include)
	}
	if len(got.Workbench.BranchPatterns) != 0 {
		t.Fatalf("expected explicit empty list to replace branch patterns, got %#v", got.Workbench.BranchPatterns)
	}
}

func TestMerge_MapsDeepMerge(t *testing.T) {
	defaultBranch := "main"
	initialRoot := "~/src/gh-zen"
	strongerRoot := "~/worktrees/gh-zen"
	present := PullRequestPresent
	dirty := LocalStatusDirty
	branchPattern := "feat/*"

	got := MergeLayers(
		PartialConfig{
			Repos: ReposConfigLayer{
				Repositories: map[string]RepositoryConfigLayer{
					"0maru/gh-zen": {
						DefaultBranch: str(defaultBranch),
						WorktreeRoot:  str(initialRoot),
					},
				},
			},
			Workbench: WorkbenchConfigLayer{
				SavedFilters: map[string]WorkbenchFilterLayer{
					"review": {
						PullRequest: ptr(present),
						LocalStatus: ptr(dirty),
					},
				},
			},
		},
		PartialConfig{
			Repos: ReposConfigLayer{
				Repositories: map[string]RepositoryConfigLayer{
					"0maru/gh-zen": {
						WorktreeRoot: str(strongerRoot),
					},
				},
			},
			Workbench: WorkbenchConfigLayer{
				SavedFilters: map[string]WorkbenchFilterLayer{
					"review": {
						BranchPattern: str(branchPattern),
					},
				},
			},
		},
	)

	repo := got.Repos.Repositories["0maru/gh-zen"]
	if repo.DefaultBranch != defaultBranch || repo.WorktreeRoot != strongerRoot {
		t.Fatalf("expected repository map to deep merge, got %+v", repo)
	}

	filter := got.Workbench.SavedFilters["review"]
	if filter.PullRequest != PullRequestPresent || filter.LocalStatus != LocalStatusDirty || filter.BranchPattern != branchPattern {
		t.Fatalf("expected saved filter map to deep merge, got %+v", filter)
	}
}

func TestMerge_DoesNotAliasBaseOrLayer(t *testing.T) {
	base := Defaults()
	base.Repos.Roots = []string{"~/src"}
	layerRoots := []string{"~/work"}
	layer := PartialConfig{
		Repos: ReposConfigLayer{Roots: &layerRoots},
		Keys:  KeyBindings{"open": {"x"}},
	}

	got := Merge(base, layer)
	base.Keys["open"][0] = "base-mutated"
	layerRoots[0] = "layer-mutated"
	layer.Keys["open"][0] = "layer-mutated"

	if !reflect.DeepEqual(got.Repos.Roots, []string{"~/work"}) {
		t.Fatalf("expected merged roots to be isolated, got %#v", got.Repos.Roots)
	}
	if !reflect.DeepEqual(got.Keys["open"], []string{"x"}) {
		t.Fatalf("expected merged keys to be isolated, got %#v", got.Keys["open"])
	}
}

func TestValidate_RejectsInvalidKnownValues(t *testing.T) {
	cfg := Defaults()
	cfg.Startup.Repo = "not-a-repo"
	cfg.Startup.View = StartupView("issues")
	cfg.UI.PreviewWidth = 1
	cfg.Keys["open"] = nil
	cfg.Repos.Roots = []string{" "}
	cfg.Workbench.Filter.PullRequest = PullRequestFilter("maybe")
	cfg.Workbench.Filter.LocalStatus = LocalStatusFilter("stale")

	err := Validate(cfg)
	if err == nil {
		t.Fatalf("expected validation error")
	}

	var validationErr ValidationError
	if !errors.As(err, &validationErr) {
		t.Fatalf("expected ValidationError, got %T", err)
	}
	for _, want := range []string{
		"startup.repo",
		"startup.view",
		"ui.preview_width",
		"keys.open",
		"repos.roots[0]",
		"workbench.filter.pull_request",
		"workbench.filter.local_status",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Fatalf("expected validation error to mention %q, got %q", want, err.Error())
		}
	}
}

func TestValidateLayer_ReturnsUnknownKeyWarnings(t *testing.T) {
	warnings, err := ValidateLayer(PartialConfig{
		UnknownKeys: []string{"ui.unused", "workbench.future"},
	})
	if err != nil {
		t.Fatalf("expected unknown keys to be non-fatal, got %v", err)
	}
	if len(warnings) != 2 {
		t.Fatalf("expected two warnings, got %#v", warnings)
	}
	if warnings[0].Path != "ui.unused" || warnings[0].Message == "" {
		t.Fatalf("unexpected warning: %+v", warnings[0])
	}
}

func TestValidateLayer_RejectsInvalidKnownValues(t *testing.T) {
	view := StartupView("issues")
	warnings, err := ValidateLayer(PartialConfig{
		Startup:     StartupConfigLayer{View: &view},
		UnknownKeys: []string{"future.option"},
	})
	if err == nil {
		t.Fatalf("expected invalid known value to be fatal")
	}
	if len(warnings) != 1 {
		t.Fatalf("expected unknown key warning to be preserved, got %#v", warnings)
	}
}

func str(value string) *string {
	return &value
}

func boolPtr(value bool) *bool {
	return &value
}

func float64Ptr(value float64) *float64 {
	return &value
}

func stringList(values ...string) *[]string {
	return &values
}

func ptr[T any](value T) *T {
	return &value
}
