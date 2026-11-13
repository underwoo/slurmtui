package ui

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"slurmtui/internal/model"
)

type jobAction uint8

const (
	actionNone jobAction = iota
	actionCancel
	actionHold
	actionRelease
	actionRequeue
)

func (action jobAction) label() string {
	switch action {
	case actionCancel:
		return "Cancel"
	case actionHold:
		return "Hold"
	case actionRelease:
		return "Release"
	case actionRequeue:
		return "Requeue"
	default:
		return "Unavailable"
	}
}

func (action jobAction) presentTense() string {
	switch action {
	case actionCancel:
		return "cancelling"
	case actionHold:
		return "holding"
	case actionRelease:
		return "releasing"
	case actionRequeue:
		return "requeueing"
	default:
		return "action"
	}
}

func (action jobAction) pastTense() string {
	switch action {
	case actionCancel:
		return "cancelled"
	case actionHold:
		return "held"
	case actionRelease:
		return "released"
	case actionRequeue:
		return "requeued"
	default:
		return "action completed for"
	}
}

func (action jobAction) command(job model.Job) (string, []string) {
	id := job.DisplayID()
	switch action {
	case actionCancel:
		return "scancel", []string{id}
	case actionHold:
		return "scontrol", []string{"hold", id}
	case actionRelease:
		return "scontrol", []string{"release", id}
	case actionRequeue:
		return "scontrol", []string{"requeue", id}
	default:
		return "", nil
	}
}

func actionForHold(job model.Job) jobAction {
	if isHeld(job) {
		return actionRelease
	}
	if hasState(job, model.StatePending) {
		return actionHold
	}
	return actionNone
}

func isHeld(job model.Job) bool {
	return hasState(job, model.StatePending) && (job.StateReason == "JobHeldUser" || job.StateReason == "JobHeldAdmin")
}

func hasState(job model.Job, state model.State) bool {
	for _, current := range job.States {
		if current == state {
			return true
		}
	}
	return false
}

func (a *App) selectedJob() model.Job {
	_, jobs, cursor, _, _ := a.activeData()
	if cursor < 0 || cursor >= len(jobs) {
		return model.Job{}
	}
	return jobs[cursor]
}

func (a *App) beginActionFromSelection(action jobAction) {
	_, jobs, cursor, _, _ := a.activeData()
	if cursor < 0 || cursor >= len(jobs) {
		return
	}
	a.beginJobAction(action, jobs[cursor], modeList)
}

func (a *App) beginJobAction(action jobAction, job model.Job, returnMode mode) {
	if action == actionNone {
		a.actionStatus = "hold/release is only available for pending jobs"
		return
	}
	a.confirmAction, a.confirmJob, a.confirmReturn = action, job, returnMode
	a.mode = modeConfirm
}

func (a *App) renderConfirm(out *strings.Builder) {
	job := a.confirmJob
	fmt.Fprintf(out, "%s\n\n", titleStyle.Render(a.confirmAction.label()+" job?"))
	fmt.Fprintf(out, "ID: %s\nName: %s\nUser: %s\nState: %s\n", job.DisplayID(), job.Name, job.UserName, jobStateText(job))
	if isNotCurrentUser(job) {
		fmt.Fprintln(out, errorStyle.Render("Warning: not your job"))
	}
	fmt.Fprintf(out, "\n%s\n", mutedStyle.Render("Press y to confirm; any other key aborts."))
}

func jobStateText(job model.Job) string {
	if len(job.States) == 0 {
		return "UNKNOWN"
	}
	return strings.Join(stateStrings(job.States), ",")
}

func stateStrings(states []model.State) []string {
	values := make([]string, len(states))
	for i, state := range states {
		values[i] = string(state)
	}
	return values
}

func isNotCurrentUser(job model.Job) bool {
	current, err := user.Current()
	name := ""
	if err == nil {
		name = current.Username
	}
	if name == "" {
		name = os.Getenv("USER")
	}
	return job.UserName != "" && name != "" && job.UserName != name
}

func (a *App) runJobAction(action jobAction, job model.Job) tea.Cmd {
	if a.runner == nil {
		return func() tea.Msg {
			return actionResult{action: action, job: job, err: fmt.Errorf("slurm runner is unavailable")}
		}
	}
	runner := a.runner
	name, args := action.command(job)
	return func() tea.Msg {
		_, err := runner.Run(context.Background(), name, args...)
		return actionResult{action: action, job: job, err: err}
	}
}

type actionResult struct {
	action jobAction
	job    model.Job
	err    error
}
