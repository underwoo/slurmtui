package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"slurmtui/internal/slurm"
)

func TestLoadFileAndCommandLineOverrides(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	content := "refresh_interval = \"30s\"\nrecent_window = \"2h\"\ndefault_scope = \"all\"\ncolumns = [\"job_id\"]\n"
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.RefreshInterval != 30*time.Second || settings.RecentWindow != 2*time.Hour || settings.DefaultScope != slurm.ScopeAll {
		t.Fatalf("loaded settings = %+v", settings)
	}
	settings, err = Override(settings, "5s", "45m", false)
	if err != nil {
		t.Fatal(err)
	}
	if settings.RefreshInterval != 5*time.Second || settings.RecentWindow != 45*time.Minute || settings.DefaultScope != slurm.ScopeAll {
		t.Fatalf("overridden settings = %+v", settings)
	}
	settings, err = Override(settings, "", "", true)
	if err != nil || settings.DefaultScope != slurm.ScopeAll {
		t.Fatalf("--all settings = %+v, err %v", settings, err)
	}
}

func TestLoadFileMissingUsesDefaults(t *testing.T) {
	settings, err := LoadFile(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if settings != Defaults() {
		t.Fatalf("defaults = %+v, want %+v", settings, Defaults())
	}
}

func TestInvalidDurationIsRejected(t *testing.T) {
	if _, err := Override(Defaults(), "0s", "", false); err == nil {
		t.Fatal("zero refresh interval was accepted")
	}
}
