package slurm

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestListRecentUsesAbsoluteTimeAndScope(t *testing.T) {
	fixedNow := time.Date(2026, 11, 11, 0, 0, 0, 0, time.FixedZone("test", -5*60*60))
	clock := func() time.Time { return fixedNow }

	t.Run("me uses caller default", func(t *testing.T) {
		runner := newFakeRunner()
		runner.addFixture(t, "sacct", []string{"--json", "-X", "-S", "2026-11-11T04:00:00"}, "sacct_recent_empty.json")
		result, err := ListRecent(context.Background(), runner, ScopeMe, time.Hour, clock)
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Jobs) != 0 || result.Jobs == nil {
			t.Fatalf("empty recent jobs = %#v, want non-nil empty slice", result.Jobs)
		}
		if got := runner.calls[0]; got != "sacct --json -X -S 2026-11-11T04:00:00" {
			t.Fatalf("command = %q", got)
		}
	})

	t.Run("all adds all-users flag", func(t *testing.T) {
		runner := newFakeRunner()
		runner.addFixture(t, "sacct", []string{"--json", "-X", "-S", "2026-11-11T04:00:00", "-a"}, "sacct_recent_empty.json")
		if _, err := ListRecent(context.Background(), runner, ScopeAll, time.Hour, clock); err != nil {
			t.Fatal(err)
		}
		if got := runner.calls[0]; !strings.HasSuffix(got, " -a") {
			t.Fatalf("all-scope command = %q, want -a", got)
		}
	})
}

func TestListRecentScrubbedFixtureNormalizesAndFilters(t *testing.T) {
	runner := newFakeRunner()
	runner.addFixture(t, "sacct", []string{"--json", "-X", "-S", "2026-11-10T23:00:00"}, "sacct_recent.json")
	result, err := ListRecent(context.Background(), runner, ScopeMe, time.Hour,
		func() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Version.String() != "25.11.7" {
		t.Fatalf("version = %s, want 25.11.7", result.Version)
	}
	if len(result.Jobs) != 4 {
		t.Fatalf("got %d recent jobs, want 4 terminal jobs (active record excluded)", len(result.Jobs))
	}
	ids := make(map[string]bool)
	for _, job := range result.Jobs {
		ids[job.DisplayID()] = true
	}
	for _, want := range []string{"34827648", "135874997", "900000_4", "910000+1"} {
		if !ids[want] {
			t.Errorf("recent job ID %q missing from %v", want, ids)
		}
	}
	if result.Jobs[0].UserName == "" || result.Jobs[0].States[0] != "COMPLETED" {
		t.Fatalf("sacct user/state were not normalized: %+v", result.Jobs[0])
	}
	if result.Jobs[0].ElapsedSeconds != 5 || result.Jobs[0].TimeLimit.Number != 210 {
		t.Fatalf("sacct timing was not normalized: elapsed=%d limit=%+v", result.Jobs[0].ElapsedSeconds, result.Jobs[0].TimeLimit)
	}
	if result.Jobs[0].EndTime.Number != 1794351631 || result.Jobs[0].StateReason != "None" {
		t.Fatalf("sacct end/reason was not normalized: end=%+v reason=%q", result.Jobs[0].EndTime, result.Jobs[0].StateReason)
	}
}

func TestListRecentErrors(t *testing.T) {
	tests := []struct {
		name string
		body string
		want string
	}{
		{name: "slurm error", body: `{"jobs":[],"meta":{"slurm":{"version":"25.11.7"}},"errors":[{"source":"slurmdbd","description":"unavailable"}]}`, want: "slurmdbd: unavailable"},
		{name: "malformed JSON", body: `{`, want: "decode sacct JSON"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			runner := newFakeRunner()
			runner.responses["sacct --json -X -S 2026-11-10T23:00:00"] = []byte(tt.body)
			_, err := ListRecent(context.Background(), runner, ScopeMe, time.Hour,
				func() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) })
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("error = %v, want substring %q", err, tt.want)
			}
		})
	}
}

func TestListRecentAccepts26_05Fixture(t *testing.T) {
	runner := newFakeRunner()
	runner.addFixture(t, "sacct", []string{"--json", "-X", "-S", "2026-11-10T23:00:00"}, "sacct_failed.json")
	result, err := ListRecent(context.Background(), runner, ScopeMe, time.Hour,
		func() time.Time { return time.Date(2026, 11, 11, 0, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	if result.Version.String() != "26.05.4" || len(result.Jobs) != 1 || result.Jobs[0].States[0] != "FAILED" {
		t.Fatalf("26.05 sacct result = version %s, jobs %+v", result.Version, result.Jobs)
	}
}
