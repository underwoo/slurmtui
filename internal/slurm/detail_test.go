package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"slurmtui/internal/model"
)

func TestGetJobDetailUsesScontrolForQueueAndDecodesFields(t *testing.T) {
	job := model.Job{ID: 42}
	runner := newFakeRunner()
	runner.responses["scontrol show job 42 --json"] = []byte(`{"jobs":[{"job_id":42,"name":"running","job_state":["RUNNING"],"comment":"  useful note  ","tres_req_str":"cpu=4,mem=8G","tres_alloc_str":"cpu=4,mem=8G"}]}`)
	detail, err := GetJobDetail(context.Background(), runner, job, false)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Job.Name != "running" || detail.Comment != "  useful note  " || detail.RequestedText != "cpu=4,mem=8G" || detail.AllocatedText != "cpu=4,mem=8G" {
		t.Fatalf("decoded detail = %+v", detail)
	}
	if len(runner.calls) != 1 || runner.calls[0] != "scontrol show job 42 --json" {
		t.Fatalf("calls = %v", runner.calls)
	}
}

func TestGetJobDetailFallsBackToSacctWhenScontrolJobIsPurged(t *testing.T) {
	runner := newFakeRunner()
	runner.errors["scontrol show job 34827648 --json"] = errors.New("slurm_load_jobs error: Invalid job id specified")
	runner.responses["sacct --json -j 34827648"] = fixture(t, "sacct_recent.json")
	detail, err := GetJobDetail(context.Background(), runner, model.Job{ID: 34827648}, false)
	if err != nil {
		t.Fatal(err)
	}
	if detail.Job.DisplayID() != "34827648" || len(detail.Steps) != 2 {
		t.Fatalf("fallback detail = %+v", detail)
	}
	if len(runner.calls) != 2 || runner.calls[1] != "sacct --json -j 34827648" {
		t.Fatalf("fallback calls = %v", runner.calls)
	}
}

func TestGetJobDetailFallsBackWhenScontrolJSONReportsInvalidJob(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["scontrol show job 34827648 --json"] = []byte(`{"jobs":[],"errors":[{"description":"Invalid job id specified","source":"slurmctld"}]}`)
	runner.responses["sacct --json -j 34827648"] = fixture(t, "sacct_recent.json")
	if _, err := GetJobDetail(context.Background(), runner, model.Job{ID: 34827648}, false); err != nil {
		t.Fatal(err)
	}
	if len(runner.calls) != 2 || runner.calls[1] != "sacct --json -j 34827648" {
		t.Fatalf("fallback calls = %v", runner.calls)
	}
}

func TestDecodeSacctDetailIncludesStepsAndPaths(t *testing.T) {
	data := bytes.Replace(fixture(t, "sacct_recent.json"), []byte(`"comment":{"administrator":"","job":"","system":""}`), []byte(`"comment":{"administrator":"private admin note","job":"  useful note  ","system":"private system note"}`), 1)
	detail, err := decodeSacctDetail(data, "34827648")
	if err != nil {
		t.Fatal(err)
	}
	if detail.Job.Name != "job-01" || detail.Comment != "  useful note  " || len(detail.Steps) != 2 {
		t.Fatalf("job = %+v, steps = %d", detail.Job, len(detail.Steps))
	}
	if detail.Steps[0].ID != "34827648.batch" || detail.Steps[1].ID != "34827648.extern" {
		t.Fatalf("step IDs = %q, %q", detail.Steps[0].ID, detail.Steps[1].ID)
	}
	if detail.Job.WorkingDirectory == "" || detail.Job.Stdout == "" || detail.Job.Command != "./job.sh" {
		t.Fatalf("paths or command missing: %+v", detail.Job)
	}
	if len(detail.Requested) == 0 || len(detail.Allocated) == 0 {
		t.Fatalf("TRES missing: requested=%v allocated=%v", detail.Requested, detail.Allocated)
	}
	if len(detail.RawJSON) == 0 {
		t.Fatal("raw detail JSON missing")
	}
}

func TestDecodeSacctDetailFailed26_05(t *testing.T) {
	detail, err := decodeSacctDetail(fixture(t, "sacct_failed.json"), "135874997")
	if err != nil {
		t.Fatal(err)
	}
	if !containsState(detail.Job.States, model.StateFailed) || len(detail.Steps) != 4 {
		t.Fatalf("state/steps = %v/%d", detail.Job.States, len(detail.Steps))
	}
}

func TestDecodeSacctDetailMatchesArrayAndHetDisplayIDs(t *testing.T) {
	for _, id := range []string{"900000_4", "910000+1"} {
		t.Run(id, func(t *testing.T) {
			detail, err := decodeSacctDetail(fixture(t, "sacct_recent.json"), id)
			if err != nil {
				t.Fatal(err)
			}
			if got := detail.Job.DisplayID(); got != id {
				t.Fatalf("display ID = %q, want %q", got, id)
			}
		})
	}
}

func TestDecodeScontrolDetailFromSyntheticPendingQueueJob(t *testing.T) {
	data := fixture(t, "squeue-01.json")
	var response struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	var job model.Job
	if err := json.Unmarshal(response.Jobs[0], &job); err != nil {
		t.Fatal(err)
	}
	if !containsState(job.States, model.StatePending) {
		t.Fatalf("first fixture job is not pending: %v", job.States)
	}
	detail, err := decodeScontrolDetail(append(append([]byte(`{"jobs":[`), response.Jobs[0]...), []byte(`]}`)...))
	if err != nil {
		t.Fatal(err)
	}
	if !containsState(detail.Job.States, model.StatePending) {
		t.Fatalf("decoded pending state = %v", detail.Job.States)
	}
}

func fixture(t *testing.T, name string) []byte {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", name)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func containsState(states []model.State, want model.State) bool {
	for _, state := range states {
		if state == want {
			return true
		}
	}
	return false
}
