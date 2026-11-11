// Package config loads the small set of user preferences used by the UI.
package config

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

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
}

func Defaults() Settings {
	return Settings{RefreshInterval: DefaultRefreshInterval, RecentWindow: DefaultRecentWindow, DefaultScope: slurm.ScopeMe}
}

// Load reads XDG_CONFIG_HOME/slurmtui/config.toml, falling back to the
// platform user config directory. A missing file keeps the built-in defaults.
func Load() (Settings, error) {
	base := os.Getenv("XDG_CONFIG_HOME")
	if base == "" {
		var err error
		base, err = os.UserConfigDir()
		if err != nil {
			return Settings{}, fmt.Errorf("find user config directory: %w", err)
		}
	}
	return LoadFile(filepath.Join(base, "slurmtui", "config.toml"))
}

func LoadFile(path string) (Settings, error) {
	settings := Defaults()
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return settings, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("open config %s: %w", path, err)
	}
	defer func() { _ = file.Close() }()

	scanner := bufio.NewScanner(file)
	for lineNumber := 1; scanner.Scan(); lineNumber++ {
		line := strings.TrimSpace(strings.SplitN(scanner.Text(), "#", 2)[0])
		if line == "" {
			continue
		}
		key, value, ok := strings.Cut(line, "=")
		if !ok {
			return Settings{}, fmt.Errorf("config %s:%d: expected key = value", path, lineNumber)
		}
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "refresh_interval", "recent_window", "default_scope":
		default:
			continue
		}
		quoted, err := strconv.Unquote(value)
		if err != nil {
			return Settings{}, fmt.Errorf("config %s:%d: expected quoted string: %w", path, lineNumber, err)
		}
		switch key {
		case "refresh_interval":
			settings.RefreshInterval, err = time.ParseDuration(quoted)
			if err != nil || settings.RefreshInterval <= 0 {
				return Settings{}, fmt.Errorf("config %s:%d: refresh_interval must be a positive duration", path, lineNumber)
			}
		case "recent_window":
			settings.RecentWindow, err = time.ParseDuration(quoted)
			if err != nil || settings.RecentWindow <= 0 {
				return Settings{}, fmt.Errorf("config %s:%d: recent_window must be a positive duration", path, lineNumber)
			}
		case "default_scope":
			switch slurm.Scope(quoted) {
			case slurm.ScopeMe, slurm.ScopeAll:
				settings.DefaultScope = slurm.Scope(quoted)
			default:
				return Settings{}, fmt.Errorf("config %s:%d: default_scope must be %q or %q", path, lineNumber, slurm.ScopeMe, slurm.ScopeAll)
			}
		}
	}
	if err := scanner.Err(); err != nil {
		return Settings{}, fmt.Errorf("read config %s: %w", path, err)
	}
	return settings, nil
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
