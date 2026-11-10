package slurm

import (
	"os"
	"path/filepath"
	"testing"
)

func TestVersionFrom26_05Sample(t *testing.T) {
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", "sacct_failed.json"))
	if err != nil {
		t.Fatal(err)
	}
	version, err := VersionFromJSON(data)
	if err != nil {
		t.Fatal(err)
	}
	if version.String() != "26.05.4" {
		t.Fatalf("version = %s, want 26.05.4", version)
	}
}
