package ui

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/teatest"
	"github.com/muesli/termenv"

	"slurmtui/internal/model"
	"slurmtui/internal/registry"
	"slurmtui/internal/slurm"
)

func TestQueueViewGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	clock := func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }
	app := NewWithOptions(nil, Options{RefreshInterval: time.Hour, RecentWindow: time.Hour, Scope: slurm.ScopeMe, Clock: clock})
	app.queueJobs = readQueueFixture(t)
	tm := teatest.NewTestModel(t, app, teatest.WithInitialTermSize(112, 16))
	t.Cleanup(func() { _ = tm.Quit() })
	teatest.WaitFor(t, tm.Output(), func(output []byte) bool {
		return bytes.Contains(output, []byte(app.queueJobs[0].Name))
	}, teatest.WithDuration(time.Second), teatest.WithCheckInterval(10*time.Millisecond))
	tm.Type("q")
	tm.FinalModel(t, teatest.WithFinalTimeout(time.Second))
	teatest.RequireEqualOutput(t, []byte(app.View()))
}

func TestSelectedJobSurvivesRefreshAndMissingJobClampsCursor(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	first := model.Job{ID: 1, Name: "first"}
	selected := model.Job{ID: 2, Name: "selected"}
	app.queueJobs = []model.Job{first, selected}
	app.queueCursor = 1
	app.Update(queueResult{scope: slurm.ScopeMe, result: slurm.ListResult{Jobs: []model.Job{selected, first}}})
	if app.queueCursor != 0 || app.queueJobs[app.queueCursor].DisplayID() != "2" {
		t.Fatalf("selection after reorder: cursor=%d jobs=%+v", app.queueCursor, app.queueJobs)
	}
	app.Update(queueResult{scope: slurm.ScopeMe, result: slurm.ListResult{Jobs: []model.Job{first}}})
	if app.queueCursor != 0 {
		t.Fatalf("cursor after selected job disappeared = %d, want clamped 0", app.queueCursor)
	}
}

func TestQueueAndRecentColumns(t *testing.T) {
	queueIDs := make(map[string]bool)
	for _, column := range registry.VisibleColumns(registry.QueueTab) {
		queueIDs[column.ID] = true
	}
	recentIDs := make(map[string]bool)
	for _, column := range registry.VisibleColumns(registry.RecentTab) {
		recentIDs[column.ID] = true
	}
	if queueIDs["end"] || !recentIDs["end"] {
		t.Fatalf("End column visibility: queue=%t recent=%t", queueIDs["end"], recentIDs["end"])
	}
	columns := registry.DefaultColumns()
	byID := make(map[string]registry.Column, len(columns))
	for _, column := range columns {
		byID[column.ID] = column
	}
	if got := byID["node_reason"].Value(model.Job{States: []model.State{model.StatePending}, StateReason: "Priority"}, time.Time{}); got != "(Priority)" {
		t.Errorf("pending NodeList(Reason) = %q, want (Priority)", got)
	}
	if got := byID["node_reason"].Value(model.Job{States: []model.State{model.StateRunning}, NodeList: "n001"}, time.Time{}); got != "n001" {
		t.Errorf("running NodeList(Reason) = %q, want n001", got)
	}
	if got := byID["elapsed_limit"].Value(model.Job{States: []model.State{model.StatePending}, TimeLimit: model.NoValNumber{Infinite: true}}, time.Time{}); got != "—/∞" {
		t.Errorf("pending Elapsed/TimeLimit = %q, want —/∞", got)
	}
}

func TestRefreshFailureKeepsExistingRows(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 1, Name: "keep"}}
	app.Update(queueResult{scope: slurm.ScopeMe, err: io.ErrUnexpectedEOF})
	if len(app.queueJobs) != 1 || app.queueJobs[0].Name != "keep" || app.queueError == nil {
		t.Fatalf("failed refresh changed existing rows: jobs=%+v err=%v", app.queueJobs, app.queueError)
	}
}

func TestDetailViewForPendingRunningCompletedAndFailedJobs(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	clock := func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }
	queue := readQueueFixture(t)
	running := readRunningFixture(t)
	cases := []struct {
		name       string
		job        model.Job
		fromRecent bool
		command    string
		response   []byte
	}{
		{name: "pending", job: queue[0], command: "scontrol show job " + queue[0].DisplayID() + " --json", response: wrapJobFixture(t, queue[0].DisplayID(), "squeue-01.json")},
		{name: "running", job: running[0], command: "scontrol show job " + running[0].DisplayID() + " --json", response: wrapJobFixture(t, running[0].DisplayID(), "squeue-02.json")},
		{name: "completed", job: model.Job{ID: 34827648, States: []model.State{model.StateCompleted}}, fromRecent: true, command: "sacct --json -j 34827648", response: readSlurmFixture(t, "sacct_recent.json")},
		{name: "failed", job: model.Job{ID: 135874997, States: []model.State{model.StateFailed}}, fromRecent: true, command: "sacct --json -j 135874997", response: readSlurmFixture(t, "sacct_failed.json")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			runner := &detailTestRunner{command: tc.command, response: tc.response}
			app := NewWithOptions(runner, Options{Scope: slurm.ScopeMe, Clock: clock})
			app.width, app.height = 120, 100
			if tc.fromRecent {
				app.tab = registry.RecentTab
				app.recentJobs = []model.Job{tc.job}
			} else {
				app.queueJobs = []model.Job{tc.job}
			}
			_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyEnter})
			if cmd == nil {
				t.Fatal("enter did not start detail fetch")
			}
			msg := cmd()
			_, _ = app.Update(msg)
			view := app.View()
			if bytes.Contains([]byte(view), []byte("All fields")) {
				t.Fatalf("All fields should be collapsed by default:\n%s", view)
			}
			teatest.RequireEqualOutput(t, []byte(view))
			_, _ = app.Update(tea.KeyMsg{Type: tea.KeyEsc})
			if app.detail != nil {
				t.Fatal("esc did not return to the job list")
			}
		})
	}
}

func TestDetailViewSplitsMultilineCommandIntoViewportRows(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.width, app.height = 120, 100
	app.detailJob = model.Job{ID: 7}
	app.detail = &model.JobDetail{Job: model.Job{ID: 7, Command: "#!/bin/sh\nprintf done\n"}}
	view := app.View()
	if !strings.Contains(view, "Command: #!/bin/sh\nprintf done\n") {
		t.Fatalf("multiline command was not split into viewport rows:\n%s", view)
	}
}

type detailTestRunner struct {
	command  string
	response []byte
}

func (r *detailTestRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	if command := name + " " + strings.Join(args, " "); command != r.command {
		return nil, fmt.Errorf("unexpected command %q", command)
	}
	return r.response, nil
}

func readRunningFixture(t *testing.T) []model.Job {
	t.Helper()
	data := readSlurmFixture(t, "squeue-02.json")
	var response struct {
		Jobs []model.Job `json:"jobs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	return response.Jobs
}

func wrapJobFixture(t *testing.T, id, filename string) []byte {
	t.Helper()
	data := readSlurmFixture(t, filename)
	var response struct {
		Jobs []json.RawMessage `json:"jobs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	for _, raw := range response.Jobs {
		var job model.Job
		if err := json.Unmarshal(raw, &job); err != nil {
			t.Fatal(err)
		}
		if job.DisplayID() == id {
			return append(append([]byte(`{"jobs":[`), raw...), []byte(`]}`)...)
		}
	}
	t.Fatalf("fixture %s has no job %s", filename, id)
	return nil
}

func readSlurmFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func readQueueFixture(t *testing.T) []model.Job {
	t.Helper()
	path := filepath.Join("..", "..", "testdata", "squeue-01.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Jobs []model.Job `json:"jobs"`
	}
	if err := json.Unmarshal(data, &response); err != nil {
		t.Fatal(err)
	}
	if len(response.Jobs) == 0 {
		t.Fatal("queue fixture has no jobs")
	}
	return response.Jobs
}

func BenchmarkRender20k(b *testing.B) {
	clock := func() time.Time { return time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC) }
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe, Clock: clock})
	app.width, app.height = 120, 30
	app.queueJobs = make([]model.Job, 20_000)
	for i := range app.queueJobs {
		app.queueJobs[i] = model.Job{ID: int64(i + 1), Name: "synthetic-job", UserName: "user", Account: "account", Partition: "compute", QOS: "normal", States: []model.State{model.StateRunning}, NodeCount: model.NoValNumber{Set: true, Number: 1}, NodeList: "node-001", StartTime: model.NoValNumber{Set: true, Number: clock().Unix() - 120}, TimeLimit: model.NoValNumber{Set: true, Number: 60}}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		_ = app.View()
	}
}

var _ tea.Model = (*App)(nil)
