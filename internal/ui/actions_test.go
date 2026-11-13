package ui

import (
	"context"
	"errors"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"slurmtui/internal/model"
	"slurmtui/internal/slurm"
)

func TestConfirmedActionsUseExactArgvAndDisplayedID(t *testing.T) {
	tests := []struct {
		name, key, command string
		args               []string
		job                model.Job
	}{
		{name: "cancel array task", key: "x", command: "scancel", args: []string{"123_4"}, job: model.Job{ID: 123, ArrayJobID: model.NoValNumber{Set: true, Number: 123}, ArrayTaskID: model.NoValNumber{Set: true, Number: 4}, States: []model.State{model.StateRunning}}},
		{name: "hold pending", key: "h", command: "scontrol", args: []string{"hold", "123"}, job: model.Job{ID: 123, States: []model.State{model.StatePending}}},
		{name: "release held", key: "h", command: "scontrol", args: []string{"release", "123"}, job: model.Job{ID: 123, States: []model.State{model.StatePending}, StateReason: "JobHeldAdmin"}},
		{name: "requeue het component", key: "R", command: "scontrol", args: []string{"requeue", "456+1"}, job: model.Job{ID: 456, HetJobID: model.NoValNumber{Set: true, Number: 456}, HetJobOffset: model.NoValNumber{Set: true, Number: 1}, States: []model.State{model.StateFailed}}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &actionTestRunner{}
			app := NewWithOptions(runner, Options{Scope: slurm.ScopeMe})
			app.queueJobs = []model.Job{tc.job}
			app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune(tc.key)})
			if app.mode != modeConfirm {
				t.Fatalf("action did not open confirmation: mode=%v", app.mode)
			}
			view := app.View()
			for _, want := range []string{tc.job.DisplayID(), tc.job.Name, "State:", "Press y to confirm"} {
				if !strings.Contains(view, want) {
					t.Errorf("confirmation omitted %q:\n%s", want, view)
				}
			}
			_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			if cmd == nil {
				t.Fatal("y did not start the action")
			}
			_, _ = app.Update(cmd())
			if len(runner.calls) != 1 || runner.calls[0].name != tc.command || strings.Join(runner.calls[0].args, " ") != strings.Join(tc.args, " ") {
				t.Fatalf("runner calls = %+v, want %s %v", runner.calls, tc.command, tc.args)
			}
			if !strings.Contains(app.actionStatus, tc.job.DisplayID()) || app.mode != modeList {
				t.Fatalf("action completion state: mode=%v status=%q", app.mode, app.actionStatus)
			}
		})
	}
}

func TestConfirmedActionAbortMakesNoRunnerCall(t *testing.T) {
	runner := &actionTestRunner{}
	app := NewWithOptions(runner, Options{})
	app.queueJobs = []model.Job{{ID: 12, States: []model.State{model.StateRunning}}}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'n'}})
	if len(runner.calls) != 0 || app.mode != modeList || !strings.Contains(app.actionStatus, "aborted") {
		t.Fatalf("abort state: calls=%+v mode=%v status=%q", runner.calls, app.mode, app.actionStatus)
	}
}

func TestConfirmationWarnsForAnotherUsersJob(t *testing.T) {
	app := NewWithOptions(nil, Options{})
	app.queueJobs = []model.Job{{ID: 8, UserName: "definitely-not-the-current-user", States: []model.State{model.StateRunning}}}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if !strings.Contains(app.View(), "not your job") {
		t.Fatalf("confirmation omitted ownership warning:\n%s", app.View())
	}
}

func TestHoldOnlyAppliesToPendingJobsAndRecognizesBothHeldReasons(t *testing.T) {
	for _, reason := range []string{"JobHeldUser", "JobHeldAdmin"} {
		job := model.Job{States: []model.State{model.StatePending}, StateReason: reason}
		if got := actionForHold(job); got != actionRelease {
			t.Errorf("actionForHold(%s) = %v, want release", reason, got)
		}
	}
	if got := actionForHold(model.Job{States: []model.State{model.StatePending}}); got != actionHold {
		t.Errorf("pending action = %v, want hold", got)
	}
	if got := actionForHold(model.Job{States: []model.State{model.StateRunning}}); got != actionNone {
		t.Errorf("running action = %v, want unavailable", got)
	}
	app := NewWithOptions(nil, Options{})
	app.queueJobs = []model.Job{{ID: 3, States: []model.State{model.StateRunning}}}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'h'}})
	if app.mode != modeList || !strings.Contains(app.actionStatus, "only available for pending") {
		t.Fatalf("non-pending hold state: mode=%v status=%q", app.mode, app.actionStatus)
	}
}

func TestActionFailureShowsRunnerError(t *testing.T) {
	runner := &actionTestRunner{err: errors.New("scancel failed: permission denied")}
	app := NewWithOptions(runner, Options{})
	app.queueJobs = []model.Job{{ID: 17, States: []model.State{model.StateRunning}}}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_, _ = app.Update(cmd())
	if !strings.Contains(app.View(), "permission denied") {
		t.Fatalf("runner error missing from status:\n%s", app.View())
	}
}

func TestActionsApplyToDetailJob(t *testing.T) {
	runner := &actionTestRunner{}
	app := NewWithOptions(runner, Options{})
	app.mode = modeDetail
	app.detailJob = model.Job{ID: 22, States: []model.State{model.StateRunning}}
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'x'}})
	if app.confirmJob.ID != app.detailJob.ID || app.confirmReturn != modeDetail {
		t.Fatalf("detail action targets wrong job: job=%+v return=%v", app.confirmJob, app.confirmReturn)
	}
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_, _ = app.Update(cmd())
	if app.mode != modeDetail || len(runner.calls) != 1 || runner.calls[0].name != "scancel" {
		t.Fatalf("detail action state: mode=%v calls=%+v", app.mode, runner.calls)
	}
}

type actionCall struct {
	name string
	args []string
}

type actionTestRunner struct {
	calls []actionCall
	err   error
}

func (r *actionTestRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, actionCall{name: name, args: append([]string(nil), args...)})
	return nil, r.err
}

var _ slurm.Runner = (*actionTestRunner)(nil)
