package slurm

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func (r *FakeRunner) addFixture(t *testing.T, name string, args []string, fixture string) {
	t.Helper()
	data, err := readFixture(fixture)
	if err != nil {
		t.Fatal(err)
	}
	r.responses[commandKey(name, args)] = data
}

func (r *FakeRunner) addQueueFixtures(t *testing.T, name string, args []string, fixtures ...string) {
	t.Helper()
	if len(fixtures) == 0 {
		t.Fatal("addQueueFixtures requires at least one fixture")
	}
	var response map[string]json.RawMessage
	var jobs []json.RawMessage
	for i, fixture := range fixtures {
		data, err := readFixture(fixture)
		if err != nil {
			t.Fatal(err)
		}
		var part map[string]json.RawMessage
		if err := json.Unmarshal(data, &part); err != nil {
			t.Fatal(err)
		}
		var partJobs []json.RawMessage
		if err := json.Unmarshal(part["jobs"], &partJobs); err != nil {
			t.Fatal(err)
		}
		jobs = append(jobs, partJobs...)
		if i == 0 {
			response = part
		}
	}
	encodedJobs, err := json.Marshal(jobs)
	if err != nil {
		t.Fatal(err)
	}
	response["jobs"] = encodedJobs
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatal(err)
	}
	r.responses[commandKey(name, args)] = data
}

func readFixture(name string) ([]byte, error) {
	return os.ReadFile(filepath.Join("..", "..", "testdata", name))
}

func TestListJobsScrubbedQueueFixture25_11(t *testing.T) {
	runner := newFakeRunner()
	runner.addQueueFixtures(t, "squeue", []string{"--json", "--me"},
		"squeue-01.json", "squeue-02.json", "squeue-03.json", "squeue-04.json")
	result, err := ListJobs(context.Background(), runner, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version.String() != "25.11.7" {
		t.Fatalf("version = %s, want 25.11.7", result.Version)
	}
	if len(result.Jobs) != 20 {
		t.Fatalf("got %d jobs, want 20", len(result.Jobs))
	}
	pending, running := 0, 0
	for _, job := range result.Jobs {
		for _, state := range job.States {
			switch state {
			case "PENDING":
				pending++
			case "RUNNING":
				running++
			}
		}
	}
	if pending != 2 || running != 18 {
		t.Fatalf("state counts: pending=%d running=%d, want 2 and 18", pending, running)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "squeue --json --me" {
		t.Fatalf("squeue calls = %v, want default --me scope", runner.calls)
	}
}

func TestListJobsEmptyZeroJobsWarningIsNotAnError(t *testing.T) {
	runner := newFakeRunner()
	runner.addFixture(t, "squeue", []string{"--json", "--me"}, "squeue_empty.json")
	result, err := ListJobs(context.Background(), runner, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Jobs) != 0 || result.Jobs == nil {
		t.Fatalf("empty queue jobs = %#v, want non-nil empty slice", result.Jobs)
	}
	if len(result.Warnings) != 0 {
		t.Fatalf("zero-jobs warning should be suppressed, got %v", result.Warnings)
	}
}
