package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/pelletier/go-toml/v2"
)

const (
	envTerminalID = "GH_ZEN_TERMINAL"

	globalConfigPath  = ".config/gh-zen/config.toml"
	projectConfigName = ".gh-zen.toml"
	terminalConfigDir = ".config/gh-zen/terminals"
)

// SourceKind identifies a loaded configuration layer.
type SourceKind string

const (
	SourceGlobal   SourceKind = "global"
	SourceProject  SourceKind = "project"
	SourceTerminal SourceKind = "terminal"
)

// LoadOptions configures config file discovery.
type LoadOptions struct {
	HomeDir    string
	ProjectDir string
	Env        map[string]string
}

// ConfigPaths contains the discovered config file paths.
type ConfigPaths struct {
	Global   string
	Project  string
	Terminal string
}

// ConfigSource records a loaded config file.
type ConfigSource struct {
	Kind SourceKind
	Path string
}

// LoadResult contains the resolved config and metadata from loading.
type LoadResult struct {
	Config      Config
	Diagnostics []Diagnostic
	Sources     []ConfigSource
}

// ResolvePaths returns the default config file paths for the provided options.
func ResolvePaths(options LoadOptions) (ConfigPaths, error) {
	homeDir, err := resolveHomeDir(options.HomeDir)
	if err != nil {
		return ConfigPaths{}, err
	}
	projectDir, err := resolveProjectDir(options.ProjectDir)
	if err != nil {
		return ConfigPaths{}, err
	}

	paths := ConfigPaths{
		Global:  filepath.Join(homeDir, filepath.FromSlash(globalConfigPath)),
		Project: filepath.Join(projectDir, projectConfigName),
	}

	if terminalID := terminalIDFromEnv(options.Env); terminalID != "" {
		if !IsSafeTerminalID(terminalID) {
			return ConfigPaths{}, fmt.Errorf("%s: unsafe terminal profile id %q", envTerminalID, terminalID)
		}
		paths.Terminal = filepath.Join(homeDir, filepath.FromSlash(terminalConfigDir), terminalID+".toml")
	}

	return paths, nil
}

// IsSafeTerminalID reports whether id can be used as a terminal profile filename.
func IsSafeTerminalID(id string) bool {
	return isSafeIdentifier(id)
}

// Load reads config files, applies discovered layers, and validates the result.
func Load(options LoadOptions) (LoadResult, error) {
	paths, err := ResolvePaths(options)
	if err != nil {
		return LoadResult{}, err
	}

	type sourcePath struct {
		kind SourceKind
		path string
	}
	sourcePaths := []sourcePath{
		{kind: SourceGlobal, path: paths.Global},
		{kind: SourceProject, path: paths.Project},
	}
	if paths.Terminal != "" {
		sourcePaths = append(sourcePaths, sourcePath{kind: SourceTerminal, path: paths.Terminal})
	}

	layers := []PartialConfig{}
	result := LoadResult{}
	for _, source := range sourcePaths {
		layer, diagnostics, loaded, err := loadLayer(source.path)
		if err != nil {
			result.Diagnostics = append(result.Diagnostics, diagnostics...)
			if loaded {
				result.Sources = append(result.Sources, ConfigSource{Kind: source.kind, Path: source.path})
			}
			return result, fmt.Errorf("%s config %q: %w", source.kind, source.path, err)
		}
		if !loaded {
			continue
		}
		layers = append(layers, layer)
		result.Diagnostics = append(result.Diagnostics, diagnostics...)
		result.Sources = append(result.Sources, ConfigSource{Kind: source.kind, Path: source.path})
	}

	result.Config = MergeLayers(layers...)
	if err := Validate(result.Config); err != nil {
		return result, err
	}
	return result, nil
}

func loadLayer(path string) (PartialConfig, []Diagnostic, bool, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return PartialConfig{}, nil, false, nil
	}
	if err != nil {
		return PartialConfig{}, nil, false, err
	}

	var layer PartialConfig
	if err := decodeLayer(data, &layer); err != nil {
		return PartialConfig{}, nil, false, err
	}
	diagnostics, err := ValidateLayer(layer)
	if err != nil {
		return PartialConfig{}, diagnostics, true, err
	}
	return layer, diagnostics, true, nil
}

func decodeLayer(data []byte, layer *PartialConfig) error {
	decoder := toml.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(layer); err != nil {
		var strictErr *toml.StrictMissingError
		if !errors.As(err, &strictErr) {
			return err
		}
		layer.UnknownKeys = strictMissingKeys(strictErr)
	}
	return nil
}

func strictMissingKeys(err *toml.StrictMissingError) []string {
	if err == nil {
		return nil
	}
	keys := make([]string, 0, len(err.Errors))
	for _, decodeErr := range err.Errors {
		if key := decodeErr.Key(); len(key) > 0 {
			keys = append(keys, strings.Join(key, "."))
		}
	}
	return keys
}

func resolveHomeDir(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return homeDir, nil
}

func resolveProjectDir(explicit string) (string, error) {
	if explicit != "" {
		return explicit, nil
	}
	projectDir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve project directory: %w", err)
	}
	return projectDir, nil
}

func terminalIDFromEnv(env map[string]string) string {
	if env != nil {
		return env[envTerminalID]
	}
	return os.Getenv(envTerminalID)
}
