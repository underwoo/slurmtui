package ui

import (
	"fmt"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"slurmtui/internal/model"
)

func TestValidTimeLimit(t *testing.T) {
	for _, value := range []string{"30", "30:45", "1:30:45", "2-03", "2-03:04", "2-03:04:05", "UNLIMITED", "unlimited"} {
		if !validTimeLimit(value) {
			t.Errorf("validTimeLimit(%q) = false", value)
		}
	}
	for _, value := range []string{"", " ", "abc", "1:2:3:4", "2-24:00", "1:60", "1:02:60", "-1", "1--2"} {
		if validTimeLimit(value) {
			t.Errorf("validTimeLimit(%q) = true", value)
		}
	}
}

func TestEditFormArgvForChangedFields(t *testing.T) {
	tests := []struct {
		name  string
		field int
		value string
		want  []string
	}{
		{name: "time limit", field: 0, value: "90", want: []string{"update", "JobId=123", "TimeLimit=90"}},
		{name: "name", field: 1, value: "two words", want: []string{"update", "JobId=123", "Name=two words"}},
		{name: "comment", field: 2, value: "stage two", want: []string{"update", "JobId=123", "Comment=stage two"}},
		{name: "combined", field: -1, value: "", want: []string{"update", "JobId=123", "TimeLimit=90", "Name=next", "Comment=stage"}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			runner := &actionTestRunner{}
			app := NewWithOptions(runner, Options{})
			job := model.Job{ID: 123, Name: "old", TimeLimit: model.NoValNumber{Set: true, Number: 60}}
			app.beginEditForm(job, "before", modeDetail)
			if tc.field >= 0 {
				app.editInputs[tc.field].SetValue(tc.value)
			} else {
				app.editInputs[0].SetValue("90")
				app.editInputs[1].SetValue("next")
				app.editInputs[2].SetValue("stage")
			}
			app.submitEditForm()
			if app.mode != modeConfirm || !app.editConfirm {
				t.Fatalf("form did not enter confirmation: mode=%v err=%q", app.mode, app.editError)
			}
			if !strings.Contains(app.View(), "→") {
				t.Fatalf("confirmation omitted before/after diff:\n%s", app.View())
			}
			if tc.name == "combined" {
				for _, want := range []string{"TimeLimit: 60 → 90", "Name: old → next", "Comment: before → stage"} {
					if !strings.Contains(app.View(), want) {
						t.Errorf("confirmation diff omitted %q:\n%s", want, app.View())
					}
				}
			}
			_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
			if cmd == nil {
				t.Fatal("confirmation did not start scontrol update")
			}
			_, _ = app.Update(cmd())
			if len(runner.calls) != 1 || runner.calls[0].name != "scontrol" || fmt.Sprint(runner.calls[0].args) != fmt.Sprint(tc.want) {
				t.Fatalf("calls = %+v, want scontrol %v", runner.calls, tc.want)
			}
		})
	}
}

func TestEditFailureShowsSlurmError(t *testing.T) {
	runner := &actionTestRunner{err: fmt.Errorf("scontrol update JobId=12 Name=other: Permission denied")}
	app := NewWithOptions(runner, Options{})
	job := model.Job{ID: 12, Name: "same", TimeLimit: model.NoValNumber{Set: true, Number: 60}}
	app.beginEditForm(job, "", modeList)
	app.editInputs[1].SetValue("other")
	app.submitEditForm()
	_, cmd := app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'y'}})
	_, _ = app.Update(cmd())
	if !strings.Contains(app.View(), "Permission denied") {
		t.Fatalf("Slurm error was not shown verbatim:\n%s", app.View())
	}
}

func TestUnchangedEditFormDoesNotCallRunner(t *testing.T) {
	runner := &actionTestRunner{}
	app := NewWithOptions(runner, Options{})
	job := model.Job{ID: 123, Name: "same", TimeLimit: model.NoValNumber{Set: true, Number: 60}}
	app.beginEditForm(job, "unchanged", modeList)
	app.submitEditForm()
	if len(runner.calls) != 0 || app.mode != modeList || app.actionStatus != "no changes for 123" {
		t.Fatalf("unchanged edit: calls=%+v mode=%v status=%q", runner.calls, app.mode, app.actionStatus)
	}
}

func TestInvalidEditTimeLimitIsRejectedLocally(t *testing.T) {
	runner := &actionTestRunner{}
	app := NewWithOptions(runner, Options{})
	app.beginEditForm(model.Job{ID: 9, TimeLimit: model.NoValNumber{Set: true, Number: 60}}, "", modeList)
	app.editInputs[0].SetValue("not-a-time")
	app.submitEditForm()
	if app.mode != modeForm || app.editError == "" || len(runner.calls) != 0 {
		t.Fatalf("invalid time limit was not rejected locally: mode=%v err=%q calls=%+v", app.mode, app.editError, runner.calls)
	}
}

func TestListEditLoadsDetailBeforePrefillingComment(t *testing.T) {
	app := NewWithOptions(nil, Options{})
	job := model.Job{ID: 55, Name: "listed", TimeLimit: model.NoValNumber{Set: true, Number: 15}}
	app.queueJobs = []model.Job{job}
	app.editAfterDetail = true
	app.mode = modeDetail
	app.detailRequest = 3
	app.Update(detailResult{request: 3, detail: model.JobDetail{Job: job, Comment: "from detail"}})
	if app.mode != modeForm || app.editInputs[1].Value() != "listed" || app.editInputs[2].Value() != "from detail" || app.editInputs[0].Value() != "15" {
		t.Fatalf("list edit form prefill: mode=%v time=%q name=%q comment=%q", app.mode, app.editInputs[0].Value(), app.editInputs[1].Value(), app.editInputs[2].Value())
	}
}

func TestEditFormTypingJDoesNotChangeField(t *testing.T) {
	app := NewWithOptions(nil, Options{})
	app.beginEditForm(model.Job{ID: 4, Name: "job", TimeLimit: model.NoValNumber{Set: true, Number: 60}}, "", modeList)
	app.editInputs[0].SetValue("1")
	app.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune{'j'}})
	if app.editField != 0 || !strings.Contains(app.editInputs[0].Value(), "j") {
		t.Fatalf("j was treated as navigation instead of text: field=%d value=%q", app.editField, app.editInputs[0].Value())
	}
}
