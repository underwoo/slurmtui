package ui

import (
	"bytes"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
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
