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

func TestLoadFileReadsColumnTables(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	if err := os.WriteFile(path, []byte("refresh_interval = \"22s\"\n[columns]\nqueue = [\"name\", \"job_id\"]\nrecent = [\"end\"]\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if settings.RefreshInterval != 22*time.Second || len(settings.QueueColumns) != 2 || settings.QueueColumns[0] != "name" || len(settings.RecentColumns) != 1 || settings.RecentColumns[0] != "end" {
		t.Fatalf("loaded settings = %+v", settings)
	}
}

func TestSaveColumnsPreservesBytesOutsideTable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	before := "# keep this comment\nrefresh_interval = \"30s\"\n\n[columns] # old selection\nqueue = [\"job_id\"]\n# old table comment\nrecent = [\"end\"]\n\n[other]\nvalue = \"keep\" # keep inline comment\n"
	if err := os.WriteFile(path, []byte(before), 0o640); err != nil {
		t.Fatal(err)
	}
	if err := SaveColumns(path, []string{"name", "job_id"}, []string{"state"}); err != nil {
		t.Fatal(err)
	}
	afterBytes, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	after := string(afterBytes)
	want := "# keep this comment\nrefresh_interval = \"30s\"\n\n[columns]\nqueue = [\"name\", \"job_id\"]\nrecent = [\"state\"]\n\n[other]\nvalue = \"keep\" # keep inline comment\n"
	if after != want {
		t.Fatalf("saved config:\n%s\nwant:\n%s", after, want)
	}
	info, err := os.Stat(path)
	if err != nil || info.Mode().Perm() != 0o640 {
		t.Fatalf("config permissions = %v, err=%v", info, err)
	}
}

func TestSaveColumnsCreatesMissingFileAndDirectory(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "config.toml")
	if err := SaveColumns(path, []string{"job_id"}, []string{"end"}); err != nil {
		t.Fatal(err)
	}
	settings, err := LoadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(settings.QueueColumns) != 1 || settings.QueueColumns[0] != "job_id" || len(settings.RecentColumns) != 1 || settings.RecentColumns[0] != "end" {
		t.Fatalf("saved settings = %+v", settings)
	}
}

func TestSaveColumnsDoesNotOverwriteBadFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.toml")
	bad := "refresh_interval = [not valid\n"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := SaveColumns(path, []string{"job_id"}, nil); err == nil {
		t.Fatal("malformed config was accepted")
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != bad {
		t.Fatalf("bad config changed to %q", after)
	}
}

func TestLoadFileMissingUsesDefaults(t *testing.T) {
	settings, err := LoadFile(filepath.Join(t.TempDir(), "missing.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if settings.RefreshInterval != Defaults().RefreshInterval || settings.RecentWindow != Defaults().RecentWindow || settings.DefaultScope != Defaults().DefaultScope || len(settings.QueueColumns) != 0 || len(settings.RecentColumns) != 0 {
		t.Fatalf("defaults = %+v, want %+v", settings, Defaults())
	}
}

func TestInvalidDurationIsRejected(t *testing.T) {
	if _, err := Override(Defaults(), "0s", "", false); err == nil {
		t.Fatal("zero refresh interval was accepted")
	}
}
