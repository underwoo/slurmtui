// Package config loads the small set of user preferences used by the UI.
package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/BurntSushi/toml"

	"slurmtui/internal/slurm"
)

const (
	DefaultRefreshInterval = 15 * time.Second
	DefaultRecentWindow    = time.Hour
)

type Settings struct {
	RefreshInterval time.Duration
	RecentWindow    time.Duration
	DefaultScope    slurm.Scope
	QueueColumns    []string
	RecentColumns   []string
	Path            string
}

func Defaults() Settings {
	return Settings{RefreshInterval: DefaultRefreshInterval, RecentWindow: DefaultRecentWindow, DefaultScope: slurm.ScopeMe}
}

// Load reads XDG_CONFIG_HOME/slurmtui/config.toml, falling back to the
// platform user config directory. A missing file keeps the built-in defaults.
func Load() (Settings, error) {
	path, err := ConfigPath()
	if err != nil {
		return Settings{}, err
	}
	settings, err := LoadFile(path)
	settings.Path = path
	return settings, err
}

func ConfigPath() (string, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return "", fmt.Errorf("find user config directory: %w", err)
		}
	}
	return filepath.Join(base, "slurmtui", "config.toml"), nil
}

func LoadFile(path string) (Settings, error) {
	settings := Defaults()
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("open config %s: %w", path, err)
	}
	var document map[string]any
	_, err = toml.Decode(string(data), &document)
	if err != nil {
		return Settings{}, fmt.Errorf("decode config %s: %w", path, err)
	}
	for key, target := range map[string]*time.Duration{"refresh_interval": &settings.RefreshInterval, "recent_window": &settings.RecentWindow} {
		value, exists := document[key]
		if !exists {
			continue
		}
		text, ok := value.(string)
		if !ok {
			return Settings{}, fmt.Errorf("config %s:%d: expected quoted string", path, keyLine(data, key))
		}
		duration, parseErr := time.ParseDuration(text)
		if parseErr != nil || duration <= 0 {
			return Settings{}, fmt.Errorf("config %s:%d: %s must be a positive duration", path, keyLine(data, key), key)
		}
		*target = duration
	}
	if value, exists := document["default_scope"]; exists {
		scope, ok := value.(string)
		if !ok {
			return Settings{}, fmt.Errorf("config %s:%d: expected quoted string", path, keyLine(data, "default_scope"))
		}
		switch slurm.Scope(scope) {
		case slurm.ScopeMe, slurm.ScopeAll:
			settings.DefaultScope = slurm.Scope(scope)
		default:
			return Settings{}, fmt.Errorf("config %s:%d: default_scope must be %q or %q", path, keyLine(data, "default_scope"), slurm.ScopeMe, slurm.ScopeAll)
		}
	}
	if rawColumns, ok := document["columns"].(map[string]any); ok {
		if values, ok := rawColumns["queue"].([]any); ok {
			settings.QueueColumns = stringValues(values)
		}
		if values, ok := rawColumns["recent"].([]any); ok {
			settings.RecentColumns = stringValues(values)
		}
	}
	return settings, nil
}

func stringValues(values []any) []string {
	result := make([]string, 0, len(values))
	for _, value := range values {
		if text, ok := value.(string); ok {
			result = append(result, text)
		}
	}
	return result
}

func keyLine(data []byte, key string) int {
	for i, line := range strings.Split(string(data), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), key+" =") {
			return i + 1
		}
	}
	return 1
}

var tableHeader = regexp.MustCompile(`^\s*\[[^\[][^]]*\]\s*(?:#.*)?$`)

// SaveColumns replaces just the top-level [columns] table, preserving all
// other bytes. The document is parsed before writing, and the replacement is
// installed with a same-directory atomic rename.
func SaveColumns(path string, queue, recent []string) error {
	original, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		original = nil
	} else if err != nil {
		return fmt.Errorf("read config %s: %w", path, err)
	}
	if len(original) != 0 {
		var document map[string]any
		if _, err := toml.Decode(string(original), &document); err != nil {
			return fmt.Errorf("decode config %s: %w", path, err)
		}
	}
	replacement := formatColumns(queue, recent)
	updated, found, err := replaceColumns(original, replacement)
	if err != nil {
		return err
	}
	if !found {
		if len(updated) > 0 && updated[len(updated)-1] != '\n' {
			updated = append(updated, '\n')
		}
		updated = append(updated, '\n')
		updated = append(updated, replacement...)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create config directory: %w", err)
	}
	mode := os.FileMode(0o600)
	if info, err := os.Stat(path); err == nil {
		mode = info.Mode().Perm()
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".config.toml-*")
	if err != nil {
		return fmt.Errorf("create temporary config: %w", err)
	}
	tempPath := tmp.Name()
	defer func() { _ = os.Remove(tempPath) }()
	if err := tmp.Chmod(mode); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("set config permissions: %w", err)
	}
	if _, err := tmp.Write(updated); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("write config: %w", err)
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return fmt.Errorf("sync config: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return fmt.Errorf("close config: %w", err)
	}
	if err := os.Rename(tempPath, path); err != nil {
		return fmt.Errorf("replace config: %w", err)
	}
	return nil
}

func formatColumns(queue, recent []string) []byte {
	var out bytes.Buffer
	fmt.Fprintln(&out, "[columns]")
	writeArray := func(name string, values []string) {
		fmt.Fprintf(&out, "%s = [", name)
		for i, value := range values {
			if i > 0 {
				fmt.Fprint(&out, ", ")
			}
			fmt.Fprintf(&out, "%q", value)
		}
		fmt.Fprintln(&out, "]")
	}
	writeArray("queue", queue)
	writeArray("recent", recent)
	return out.Bytes()
}

func replaceColumns(original, replacement []byte) ([]byte, bool, error) {
	lines := bytes.SplitAfter(original, []byte("\n"))
	start, end, offset := -1, -1, 0
	for i, line := range lines {
		trimmed := strings.TrimSpace(strings.TrimSuffix(string(line), "\n"))
		if start < 0 && isColumnsHeader(trimmed) {
			start = offset
			end = i + 1
			offset += len(line)
			continue
		}
		if start >= 0 {
			if tableHeader.MatchString(trimmed) {
				end = i
				break
			}
			end = i + 1
		}
		offset += len(line)
	}
	if start < 0 {
		return append([]byte(nil), original...), false, nil
	}
	for end > 0 && end <= len(lines) && strings.TrimSpace(string(lines[end-1])) == "" {
		end--
	}
	endOffset := 0
	for i := 0; i < end; i++ {
		endOffset += len(lines[i])
	}
	updated := make([]byte, 0, len(original)-(endOffset-start)+len(replacement))
	updated = append(updated, original[:start]...)
	updated = append(updated, replacement...)
	updated = append(updated, original[endOffset:]...)
	return updated, true, nil
}

func isColumnsHeader(line string) bool {
	withoutComment := strings.TrimSpace(strings.SplitN(line, "#", 2)[0])
	return withoutComment == "[columns]"
}

// Override applies explicitly supplied command-line values to file settings.
func Override(settings Settings, refresh, window string, all bool) (Settings, error) {
	if refresh != "" {
		duration, err := time.ParseDuration(refresh)
		if err != nil || duration <= 0 {
			return Settings{}, fmt.Errorf("invalid --refresh value %q: expected a positive duration", refresh)
		}
		settings.RefreshInterval = duration
	}
	if window != "" {
		duration, err := time.ParseDuration(window)
		if err != nil || duration <= 0 {
			return Settings{}, fmt.Errorf("invalid --window value %q: expected a positive duration", window)
		}
		settings.RecentWindow = duration
	}
	if all {
		settings.DefaultScope = slurm.ScopeAll
	}
	return settings, nil
}
