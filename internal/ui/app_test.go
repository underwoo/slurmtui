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

func TestModeKeysDoNotLeak(t *testing.T) {
	app := NewWithOptions(nil, Options{})
	app.mode = modeFilter
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'q'}})
	if cmd != nil {
		t.Fatal("typing q in filter mode unexpectedly triggered a command")
	}
	if app.mode != modeFilter {
		t.Fatalf("typing q changed mode to %v", app.mode)
	}
}

func TestFilterSmartCase(t *testing.T) {
	insensitive, err := compileFilter("alpha")
	if err != nil {
		t.Fatal(err)
	}
	if !insensitive.MatchString("ALPHA") {
		t.Fatal("lowercase pattern should match case-insensitively")
	}
	sensitive, err := compileFilter("Alpha")
	if err != nil {
		t.Fatal(err)
	}
	if sensitive.MatchString("alpha") || !sensitive.MatchString("Alpha") {
		t.Fatal("pattern with uppercase letter should match case-sensitively")
	}
	jobs := []model.Job{{ID: 1, Name: "ALPHA"}}
	if got := filterJobs(jobs, registry.VisibleColumns(registry.QueueTab), insensitive, time.Time{}, time.UTC); len(got) != 1 {
		t.Fatalf("smart-case filter matched %d rows, want 1", len(got))
	}
}

func TestFilterInputAppliesLiveAndEscapeClears(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}
	app.beginFilter()
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'a'}})
	if app.mode != modeFilter || app.filterPattern != "a" || len(app.visibleQueue()) != 2 {
		t.Fatalf("filter did not apply live: mode=%v pattern=%q jobs=%+v", app.mode, app.filterPattern, app.visibleQueue())
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.mode != modeList || app.filterPattern != "a" {
		t.Fatalf("enter did not keep filter: mode=%v pattern=%q", app.mode, app.filterPattern)
	}
	app.Update(tea.KeyMsg{Type: tea.KeyEsc})
	if app.filterPattern != "" || app.filterRegex != nil || len(app.visibleQueue()) != 2 {
		t.Fatalf("escape did not clear filter: pattern=%q jobs=%+v", app.filterPattern, app.visibleQueue())
	}
}

func TestFilterInvalidPatternKeepsLastValidFilter(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 1, Name: "alpha"}, {ID: 2, Name: "beta"}}
	app.applyFilter("alpha")
	lastValid := app.filterRegex
	app.applyFilter("[")
	if app.filterError == nil {
		t.Fatal("invalid pattern produced no error")
	}
	if app.filterRegex != lastValid || len(app.visibleQueue()) != 1 || app.visibleQueue()[0].ID != 1 {
		t.Fatalf("invalid pattern changed last valid results: regex=%p jobs=%+v", app.filterRegex, app.visibleQueue())
	}
}

func TestFilterNoMatchesAndSurvivesRefresh(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 1, Name: "alpha"}, {ID: 2, Name: "worker-two"}, {ID: 3, Name: "worker-three"}}
	app.applyFilter("missing")
	if len(app.visibleQueue()) != 0 {
		t.Fatalf("no-match filter returned jobs: %+v", app.visibleQueue())
	}
	if !strings.Contains(app.View(), "0 of 3") {
		t.Fatalf("filter status omitted no-match count:\n%s", app.View())
	}
	app.applyFilter("worker")
	app.queueCursor = 1 // select job 3 in the filtered set
	app.Update(queueResult{scope: slurm.ScopeMe, result: slurm.ListResult{Jobs: []model.Job{{ID: 1, Name: "alpha"}, {ID: 3, Name: "worker-three"}, {ID: 2, Name: "worker-two"}}}})
	if app.filterPattern != "worker" || len(app.visibleQueue()) != 2 {
		t.Fatalf("filter did not survive refresh: pattern=%q jobs=%+v", app.filterPattern, app.visibleQueue())
	}
	if got := app.visibleQueue()[app.queueCursor].ID; got != 3 {
		t.Fatalf("selection after refresh = %d, want 3", got)
	}
}

func TestSortByColumnFlipsDirectionAndKeepsSelection(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 3, Name: "c"}, {ID: 1, Name: "a"}, {ID: 2, Name: "b"}}
	app.queueCursor = 0
	app.beginSort()
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if app.queueSort.columnID != "job_id" || app.queueSort.descending {
		t.Fatalf("first sort selection = %+v, want ascending JobID", app.queueSort)
	}
	if got := app.visibleQueue()[app.queueCursor].ID; got != 3 {
		t.Fatalf("ascending sort selection = %d, want selected job 3", got)
	}
	if !strings.Contains(app.View(), "JobID ▲") {
		t.Fatalf("ascending sort indicator missing:\n%s", app.View())
	}
	app.beginSort()
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !app.queueSort.descending {
		t.Fatal("selecting current sort column did not flip direction")
	}
	if got := app.visibleQueue()[app.queueCursor].ID; got != 3 {
		t.Fatalf("descending sort selection = %d, want selected job 3", got)
	}
	if !strings.Contains(app.View(), "JobID ▼") {
		t.Fatalf("descending sort indicator missing:\n%s", app.View())
	}
}

func TestSortIsPerTabAndUsesJobIDForTies(t *testing.T) {
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe})
	app.queueJobs = []model.Job{{ID: 3, Name: "same"}, {ID: 1, Name: "same"}, {ID: 2, Name: "same"}}
	app.beginSort()
	app.picker.Move(1) // Name
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	got := []int64{app.queueJobs[0].ID, app.queueJobs[1].ID, app.queueJobs[2].ID}
	if fmt.Sprint(got) != "[1 2 3]" {
		t.Fatalf("equal values were not broken by JobID: %v", got)
	}
	app.tab = registry.RecentTab
	app.recentJobs = []model.Job{{ID: 8}, {ID: 7}}
	if app.recentSort.active {
		t.Fatal("Queue sort state leaked into Recent")
	}
	if got := app.recentJobs[0].ID; got != 8 {
		t.Fatalf("Recent source order changed without its own sort: first ID=%d", got)
	}
	app.beginSort()
	app.Update(tea.KeyMsg{Type: tea.KeyEnter})
	if !app.recentSort.active || app.recentSort.columnID != "job_id" || app.queueSort.columnID != "name" {
		t.Fatalf("sort state not isolated per tab: queue=%+v recent=%+v", app.queueSort, app.recentSort)
	}
	if got := app.recentJobs[0].ID; got != 7 {
		t.Fatalf("Recent sort did not apply independently: first ID=%d", got)
	}
}

func TestRecentViewGolden(t *testing.T) {
	lipgloss.SetColorProfile(termenv.Ascii)
	location := time.FixedZone("EST", -5*60*60)
	app := NewWithOptions(nil, Options{Scope: slurm.ScopeMe, Location: location})
	app.width, app.height = 180, 16
	app.tab = registry.RecentTab
	app.recentJobs = []model.Job{{
		ID: 34827648, Name: "completed", UserName: "user", Account: "account",
		Partition: "partition", QOS: "qos", States: []model.State{model.StateCompleted},
		EndTime: model.NoValNumber{Set: true, Number: 1794351631},
	}}
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
	var endColumn registry.Column
	for _, column := range registry.DefaultColumns() {
		if column.ID == "end" {
			endColumn = column
			break
		}
	}
	jobWithEnd := model.Job{EndTime: model.NoValNumber{Set: true, Number: 1794351631}}
	utcTime := endColumn.Value(jobWithEnd, time.Time{}, time.UTC)
	estTime := endColumn.Value(jobWithEnd, time.Time{}, time.FixedZone("EST", -5*60*60))
	if utcTime != "11-10 23:00" || estTime != "11-10 18:00" {
		t.Fatalf("End column time: UTC=%q EST=%q, want 11-10 23:00 / 11-10 18:00", utcTime, estTime)
	}
	columns := registry.DefaultColumns()
	byID := make(map[string]registry.Column, len(columns))
	for _, column := range columns {
		byID[column.ID] = column
	}
	if got := byID["node_reason"].Value(model.Job{States: []model.State{model.StatePending}, StateReason: "Priority"}, time.Time{}, nil); got != "(Priority)" {
		t.Errorf("pending NodeList(Reason) = %q, want (Priority)", got)
	}
	if got := byID["node_reason"].Value(model.Job{States: []model.State{model.StateRunning}, NodeList: "n001"}, time.Time{}, nil); got != "n001" {
		t.Errorf("running NodeList(Reason) = %q, want n001", got)
	}
	if got := byID["elapsed_limit"].Value(model.Job{States: []model.State{model.StatePending}, TimeLimit: model.NoValNumber{Infinite: true}}, time.Time{}, nil); got != "—/∞" {
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
	location := time.FixedZone("EST", -5*60*60)
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
			app := NewWithOptions(runner, Options{Scope: slurm.ScopeMe, Clock: clock, Location: location})
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
	app.mode = modeDetail
	app.detailJob = model.Job{ID: 7}
	app.detail = &model.JobDetail{Job: model.Job{ID: 7, Command: "#!/bin/sh\nprintf done\n"}}
	view := app.View()
	if !strings.Contains(view, "Command: #!/bin/sh\nprintf done\n") {
		t.Fatalf("multiline command was not split into viewport rows:\n%s", view)
	}
}

func TestWrapCommentLineRespectsWidthAndNewlines(t *testing.T) {
	got := wrapCommentLine("Comment: first long comment\nsecond line", 20)
	want := []string{"Comment: first long", "         comment", "         second line"}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("wrapped comment = %q, want %q", got, want)
	}
}

func TestSummaryTrimsCommentsAndOmitsBlankValues(t *testing.T) {
	var summary registry.DetailSection
	for _, section := range registry.DetailSections() {
		if section.ID == "summary" {
			summary = section
			break
		}
	}
	if summary.Render == nil {
		t.Fatal("summary detail section not found")
	}
	for _, comment := range []string{"", " \n\t "} {
		for _, line := range summary.Render(model.JobDetail{Comment: comment}, time.Time{}, nil) {
			if strings.HasPrefix(line, "Comment:") {
				t.Fatalf("blank comment rendered as %q", line)
			}
		}
	}
	for _, line := range summary.Render(model.JobDetail{Comment: "  note\ncontinued  "}, time.Time{}, nil) {
		if line == "Comment: note\ncontinued" {
			return
		}
	}
	t.Fatal("summary did not trim surrounding whitespace while retaining an internal newline")
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

func BenchmarkFilter20k(b *testing.B) {
	clock := time.Date(2026, 11, 11, 12, 0, 0, 0, time.UTC)
	jobs := make([]model.Job, 20_000)
	for i := range jobs {
		jobs[i] = model.Job{ID: int64(i + 1), Name: "synthetic-job", UserName: "user", Account: "account", Partition: "compute", QOS: "normal", States: []model.State{model.StateRunning}, NodeCount: model.NoValNumber{Set: true, Number: 1}, NodeList: "node-001", StartTime: model.NoValNumber{Set: true, Number: clock.Unix() - 120}, TimeLimit: model.NoValNumber{Set: true, Number: 60}}
	}
	columns := registry.VisibleColumns(registry.QueueTab)
	b.ReportAllocs()
	b.ResetTimer()
	for range b.N {
		pattern, err := compileFilter("zzzz-not-found")
		if err != nil {
			b.Fatal(err)
		}
		_ = filterJobs(jobs, columns, pattern, clock, time.UTC)
	}
}

var _ tea.Model = (*App)(nil)
