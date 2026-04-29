package config

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestResolvePaths_UsesDefaultConfigLocations(t *testing.T) {
	homeDir := filepath.Join("tmp", "home")
	projectDir := filepath.Join("tmp", "project")

	got, err := ResolvePaths(LoadOptions{
		HomeDir:    homeDir,
		ProjectDir: projectDir,
		Env:        map[string]string{},
	})
	if err != nil {
		t.Fatalf("expected paths to resolve, got %v", err)
	}

	want := ConfigPaths{
		Global:  filepath.Join(homeDir, ".config", "gh-zen", "config.toml"),
		Project: filepath.Join(projectDir, ".gh-zen.toml"),
	}
	if got != want {
		t.Fatalf("expected paths %+v, got %+v", want, got)
	}
}

func TestResolvePaths_AddsTerminalProfilePath(t *testing.T) {
	homeDir := filepath.Join("tmp", "home")
	projectDir := filepath.Join("tmp", "project")

	got, err := ResolvePaths(LoadOptions{
		HomeDir:    homeDir,
		ProjectDir: projectDir,
		Env:        map[string]string{envTerminalID: "agent-a_1"},
	})
	if err != nil {
		t.Fatalf("expected paths to resolve, got %v", err)
	}

	want := filepath.Join(homeDir, ".config", "gh-zen", "terminals", "agent-a_1.toml")
	if got.Terminal != want {
		t.Fatalf("expected terminal path %q, got %q", want, got.Terminal)
	}
}

func TestResolvePaths_RejectsUnsafeTerminalIDs(t *testing.T) {
	cases := []string{
		"../agent",
		"agent/a",
		"agent a",
		".agent",
	}
	for _, terminalID := range cases {
		t.Run(terminalID, func(t *testing.T) {
			_, err := ResolvePaths(LoadOptions{
				HomeDir:    "home",
				ProjectDir: "project",
				Env:        map[string]string{envTerminalID: terminalID},
			})
			if err == nil {
				t.Fatalf("expected unsafe terminal id %q to fail", terminalID)
			}
			if !strings.Contains(err.Error(), envTerminalID) {
				t.Fatalf("expected error to mention %s, got %v", envTerminalID, err)
			}
		})
	}
}

func TestIsSafeTerminalID(t *testing.T) {
	cases := []struct {
		value string
		want  bool
	}{
		{value: "agent-a_1", want: true},
		{value: "AgentA", want: true},
		{value: "", want: false},
		{value: "../agent", want: false},
		{value: "agent.toml", want: false},
		{value: "agent/a", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.value, func(t *testing.T) {
			if got := IsSafeTerminalID(tc.value); got != tc.want {
				t.Fatalf("expected %q safety to be %v, got %v", tc.value, tc.want, got)
			}
		})
	}
}

func TestLoad_MissingFilesUseDefaults(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary filesystem config files")
	}

	result, err := Load(LoadOptions{
		HomeDir:    t.TempDir(),
		ProjectDir: t.TempDir(),
		Env:        map[string]string{},
	})
	if err != nil {
		t.Fatalf("expected missing config files to be absent, got %v", err)
	}
	if !reflect.DeepEqual(result.Config, Defaults()) {
		t.Fatalf("expected defaults for missing files, got %+v", result.Config)
	}
	if len(result.Sources) != 0 {
		t.Fatalf("expected no loaded sources, got %+v", result.Sources)
	}
}

func TestLoad_ConfigFilesMergeInDiscoveryOrder(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary filesystem config files")
	}

	homeDir := t.TempDir()
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(homeDir, ".config", "gh-zen", "config.toml"), `
[startup]
repo = "0maru/global"
view = "workbench"

[ui]
theme = "global"
preview_width = 0.30
show_icons = false

[keys]
open = ["o"]
`)
	writeFile(t, filepath.Join(projectDir, ".gh-zen.toml"), `
[startup]
repo = "0maru/project"

[ui]
theme = "project"

[repos]
roots = ["~/project"]
`)
	writeFile(t, filepath.Join(homeDir, ".config", "gh-zen", "terminals", "agent-a.toml"), `
[startup]
repo = "0maru/terminal"

[ui]
preview_width = 0.60

[workbench]
branch_patterns = ["agent/*"]
`)

	result, err := Load(LoadOptions{
		HomeDir:    homeDir,
		ProjectDir: projectDir,
		Env:        map[string]string{envTerminalID: "agent-a"},
	})
	if err != nil {
		t.Fatalf("expected config to load, got %v", err)
	}

	cfg := result.Config
	if cfg.Startup.Repo != "0maru/terminal" {
		t.Fatalf("expected terminal startup repo override, got %q", cfg.Startup.Repo)
	}
	if cfg.UI.Theme != "project" {
		t.Fatalf("expected project theme to override global, got %q", cfg.UI.Theme)
	}
	if cfg.UI.PreviewWidth != 0.60 {
		t.Fatalf("expected terminal preview width override, got %.2f", cfg.UI.PreviewWidth)
	}
	if cfg.UI.ShowIcons {
		t.Fatalf("expected global show_icons=false to remain")
	}
	if !reflect.DeepEqual(cfg.Keys["open"], []string{"o"}) {
		t.Fatalf("expected global key binding override, got %#v", cfg.Keys["open"])
	}
	if !reflect.DeepEqual(cfg.Repos.Roots, []string{"~/project"}) {
		t.Fatalf("expected project repo roots, got %#v", cfg.Repos.Roots)
	}
	if !reflect.DeepEqual(cfg.Workbench.BranchPatterns, []string{"agent/*"}) {
		t.Fatalf("expected terminal branch patterns, got %#v", cfg.Workbench.BranchPatterns)
	}

	wantSources := []SourceKind{SourceGlobal, SourceProject, SourceTerminal}
	gotSources := []SourceKind{}
	for _, source := range result.Sources {
		gotSources = append(gotSources, source.Kind)
	}
	if !reflect.DeepEqual(gotSources, wantSources) {
		t.Fatalf("expected source order %#v, got %#v", wantSources, gotSources)
	}
}

func TestLoad_RejectsInvalidLoadedConfig(t *testing.T) {
	if testing.Short() {
		t.Skip("uses temporary filesystem config files")
	}

	homeDir := t.TempDir()
	projectDir := t.TempDir()
	writeFile(t, filepath.Join(projectDir, ".gh-zen.toml"), `
[ui]
preview_width = 2.0
`)

	_, err := Load(LoadOptions{
		HomeDir:    homeDir,
		ProjectDir: projectDir,
		Env:        map[string]string{},
	})
	if err == nil {
		t.Fatalf("expected invalid loaded config to fail")
	}
	if !strings.Contains(err.Error(), "ui.preview_width") {
		t.Fatalf("expected error to mention ui.preview_width, got %v", err)
	}
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
