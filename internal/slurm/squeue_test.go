package slurm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestListJobsAcceptsVersion26_05AndStringVersion(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[{"job_id":42,"name":"test","job_state":["FAILED"]}],"meta":{"slurm":{"version":"26.05.4"}},"errors":[],"warnings":[]}`)
	result, err := ListJobs(context.Background(), runner, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if result.Version.String() != "26.05.4" {
		t.Fatalf("version = %s, want 26.05.4", result.Version)
	}
	if len(result.Jobs) != 1 || result.Jobs[0].States[0] != "FAILED" {
		t.Fatalf("unexpected decoded job: %+v", result.Jobs)
	}
}

func TestListJobsRejectsUnsupportedVersion(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[],"meta":{"slurm":{"version":{"major":"25","minor":"10","micro":"9"}}}}`)
	_, err := ListJobs(context.Background(), runner, ScopeMe)
	if err == nil || !strings.Contains(err.Error(), "requires Slurm 25.11 or newer") {
		t.Fatalf("error = %v, want minimum-version message", err)
	}
}

func TestListJobsMissingVersion(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[],"meta":{"slurm":{}}}`)
	_, err := ListJobs(context.Background(), runner, ScopeMe)
	if err == nil || !strings.Contains(err.Error(), "missing meta.slurm.version") {
		t.Fatalf("error = %v, want missing-version message", err)
	}
}

func TestListJobsSurfacesOtherWarnings(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[],"meta":{"slurm":{"version":{"major":"25","minor":"11","micro":"7"}}},"warnings":[{"description":"partial data","source":"squeue"}]}`)
	result, err := ListJobs(context.Background(), runner, ScopeMe)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Warnings) != 1 || result.Warnings[0] != "squeue: partial data" {
		t.Fatalf("warnings = %v, want surfaced warning", result.Warnings)
	}
}

func TestListJobsSurfacesResponseErrors(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[],"meta":{"slurm":{"version":{"major":"25","minor":"11","micro":"7"}}},"errors":[{"description":"controller unavailable","source":"slurmctld"}]}`)
	_, err := ListJobs(context.Background(), runner, ScopeMe)
	if err == nil || !strings.Contains(err.Error(), "slurmctld: controller unavailable") {
		t.Fatalf("error = %v, want Slurm response error", err)
	}
}

func TestListJobsMalformedJSON(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json --me"] = []byte(`{"jobs":[`)
	_, err := ListJobs(context.Background(), runner, ScopeMe)
	if err == nil || !strings.Contains(err.Error(), "decode squeue JSON") {
		t.Fatalf("error = %v, want malformed JSON error", err)
	}
}

func TestListJobsAllScopeOmitsMe(t *testing.T) {
	runner := newFakeRunner()
	runner.responses["squeue --json"] = []byte(`{"jobs":[],"meta":{"slurm":{"version":{"major":"25","minor":"11","micro":"7"}}}}`)
	if _, err := ListJobs(context.Background(), runner, ScopeAll); err != nil {
		t.Fatal(err)
	}
	if runner.calls[0] != "squeue --json" {
		t.Fatalf("all-scope command = %q, want no --me", runner.calls[0])
	}
}

func TestExecRunnerMissingBinary(t *testing.T) {
	_, err := (ExecRunner{}).Run(context.Background(), "slurmtui-test-command-does-not-exist")
	if err == nil {
		t.Fatal("expected missing-binary error")
	}
}

func TestExecRunnerNonzeroExitIncludesStderr(t *testing.T) {
	_, err := (ExecRunner{}).Run(context.Background(), "sh", "-c", "echo synthetic-failure >&2; exit 7")
	if err == nil || !strings.Contains(err.Error(), "synthetic-failure") {
		t.Fatalf("error = %v, want stderr from failed command", err)
	}
}

func TestExecRunnerTimeout(t *testing.T) {
	_, err := (ExecRunner{Timeout: 25 * time.Millisecond}).Run(context.Background(), "sh", "-c", "exec sleep 2")
	if err == nil || !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error = %v, want timeout error", err)
	}
}
