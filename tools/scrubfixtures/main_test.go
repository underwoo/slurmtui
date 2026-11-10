package main

import (
	"encoding/json"
	"testing"
)

func TestPrioritizePendingRetainsPendingAndStableOrder(t *testing.T) {
	jobs := []any{
		map[string]any{"job_id": 1, "job_state": []any{"RUNNING"}},
		map[string]any{"job_id": 2, "job_state": []any{"PENDING"}},
		map[string]any{"job_id": 3, "job_state": []any{"COMPLETED"}},
		map[string]any{"job_id": 4, "job_state": []any{"PENDING"}},
	}
	got := prioritizePending(jobs, 3)
	if len(got) != 3 {
		t.Fatalf("got %d jobs, want 3", len(got))
	}
	ids := make([]int, 0, len(got))
	for _, raw := range got {
		ids = append(ids, int(raw.(map[string]any)["job_id"].(int)))
	}
	if len(ids) != 3 || ids[0] != 2 || ids[1] != 4 || ids[2] != 1 {
		t.Fatalf("selected job IDs = %v, want [2 4 1]", ids)
	}
}

func TestSliceJobsKeepsRootMetadata(t *testing.T) {
	root := map[string]any{
		"meta": map[string]any{"cluster": "cluster-01"},
		"jobs": []any{1, 2, 3, 4, 5, 6},
	}
	got := sliceJobs(root, 2, 3).(map[string]any)
	jobs := got["jobs"].([]any)
	if len(jobs) != 3 || jobs[0] != 3 || jobs[2] != 5 {
		t.Fatalf("sliced jobs = %v, want [3 4 5]", jobs)
	}
	if got["meta"] == nil {
		t.Fatal("slice removed response metadata")
	}
}

func TestWithoutJobResourcesRemovesOnlyThatField(t *testing.T) {
	root := map[string]any{"jobs": []any{map[string]any{
		"job_id":        1,
		"job_resources": map[string]any{"nodes": "private-node-list"},
		"name":          "kept",
	}}}
	got := withoutJobResources(root).(map[string]any)["jobs"].([]any)[0].(map[string]any)
	if _, ok := got["job_resources"]; ok {
		t.Fatal("job_resources was retained")
	}
	if got["job_id"] != 1 || got["name"] != "kept" {
		t.Fatalf("unrelated job fields changed: %v", got)
	}
}

func TestScrubberReplacesSensitiveValuesConsistently(t *testing.T) {
	input := map[string]any{
		"jobs": []any{map[string]any{
			"job_id":                    7,
			"job_state":                 []any{"PENDING"},
			"user_name":                 "private-user",
			"mail_user":                 "private-user",
			"user_id":                   json.Number("7312"),
			"group_id":                  json.Number("812"),
			"account":                   "private-account",
			"current_working_directory": "/private/home/private-user/run",
			"standard_output":           "/private/home/private-user/run/output.txt",
			"submit_line":               "sbatch --mail-user=private-user private-job.sh",
			"script":                    "echo private-user",
		}},
	}
	cleaner := newScrubber()
	cleaned := cleaner.cleanRoot(input, 1).(map[string]any)
	job := cleaned["jobs"].([]any)[0].(map[string]any)
	if job["user_name"] != "user-01" || job["mail_user"] != "user-01" {
		t.Fatalf("user fields were not consistently anonymized: %v / %v", job["user_name"], job["mail_user"])
	}
	if job["account"] == "private-account" || job["user_id"] == json.Number("7312") || job["group_id"] == json.Number("812") {
		t.Fatal("account or numeric identity remained in scrubbed output")
	}
	if job["current_working_directory"] != "/tmp/slurmtui-fixture/path-01" || job["standard_output"] == "/private/home/private-user/run/output.txt" {
		t.Fatal("path fields were not replaced")
	}
	if job["submit_line"] != "./job.sh" || job["script"] != "#!/bin/sh\n# scrubbed job script\n" {
		t.Fatal("script-bearing fields were not replaced")
	}
}

func TestPrioritizePendingTrimsAndRetainsPending(t *testing.T) {
	jobs := []any{
		map[string]any{"job_id": 1, "job_state": []any{"RUNNING"}},
		map[string]any{"job_id": 2, "job_state": []any{"PENDING"}},
		map[string]any{"job_id": 3, "job_state": []any{"RUNNING"}},
		map[string]any{"job_id": 4, "job_state": []any{"PENDING"}},
		map[string]any{"job_id": 5, "job_state": []any{"RUNNING"}},
	}
	got := prioritizePending(jobs, 3)
	if len(got) != 3 {
		t.Fatalf("got %d jobs, want 3", len(got))
	}
	ids := make(map[int]bool)
	for _, raw := range got {
		ids[int(raw.(map[string]any)["job_id"].(int))] = true
	}
	for _, id := range []int{2, 4} {
		if !ids[id] {
			t.Errorf("pending job %d was dropped", id)
		}
	}
}

func TestScrubberClearsNestedComments(t *testing.T) {
	cleaner := newScrubber()
	value := cleaner.clean("comment", map[string]any{
		"administrator": "private admin note",
		"job":           "private job note",
		"system":        "private system note",
	}).(map[string]any)
	for key, item := range value {
		if item != "" {
			t.Errorf("comment field %q was not cleared", key)
		}
	}
}

func TestScrubberIsDeterministic(t *testing.T) {
	input := []byte(`{"jobs":[{"job_id":1,"job_state":["PENDING"],"account":"a2","user_name":"u2","standard_output":"/x/out","current_working_directory":"/x"},{"job_id":2,"job_state":["RUNNING"],"account":"a1","user_name":"u1","standard_output":"/y/out","current_working_directory":"/y"}]}`)
	clean := func() []byte {
		var value any
		if err := json.Unmarshal(input, &value); err != nil {
			t.Fatal(err)
		}
		value = newScrubber().cleanRoot(value, 0)
		data, err := json.Marshal(value)
		if err != nil {
			t.Fatal(err)
		}
		return data
	}
	first, second := clean(), clean()
	if string(first) != string(second) {
		t.Fatal("scrubbed output changed between runs")
	}
}
