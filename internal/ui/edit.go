package ui

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"

	"slurmtui/internal/model"
)

func (a *App) beginEditForm(job model.Job, comment string, returnMode mode) {
	a.editJob, a.editComment, a.editReturn = job, comment, returnMode
	a.editError, a.editConfirm, a.editDiff = "", false, nil
	values := []string{timeLimitValue(job.TimeLimit), job.Name, comment}
	labels := []string{"TimeLimit: ", "Name: ", "Comment: "}
	for i := range a.editInputs {
		input := textinput.New()
		input.Prompt = labels[i]
		input.Width = max(1, a.width-20)
		input.CharLimit = 1024
		input.SetValue(values[i])
		input.CursorEnd()
		a.editInputs[i] = input
	}
	a.editField = 0
	a.editInputs[0].Focus()
	a.mode = modeForm
}

func timeLimitValue(limit model.NoValNumber) string {
	if limit.Infinite {
		return "UNLIMITED"
	}
	if !limit.Set {
		return ""
	}
	return strconv.FormatInt(limit.Number, 10)
}

func (a *App) focusEditField(index int) {
	a.editField = max(0, min(len(a.editInputs)-1, index))
	for i := range a.editInputs {
		if i == a.editField {
			a.editInputs[i].Focus()
		} else {
			a.editInputs[i].Blur()
		}
	}
}

func (a *App) submitEditForm() {
	if !validTimeLimit(a.editInputs[0].Value()) {
		a.editError = "invalid TimeLimit; use MM, MM:SS, HH:MM:SS, D-HH[:MM[:SS]], or UNLIMITED"
		return
	}
	labels := []string{"TimeLimit", "Name", "Comment"}
	old := []string{timeLimitValue(a.editJob.TimeLimit), a.editJob.Name, a.editComment}
	newValues := []string{a.editInputs[0].Value(), a.editInputs[1].Value(), a.editInputs[2].Value()}
	a.editDiff = a.editDiff[:0]
	for i := range labels {
		if old[i] != newValues[i] {
			a.editDiff = append(a.editDiff, fmt.Sprintf("%s: %s → %s", labels[i], showEmpty(old[i]), showEmpty(newValues[i])))
		}
	}
	if len(a.editDiff) == 0 {
		a.editConfirm = false
		a.mode = a.editReturn
		a.actionStatus = "no changes for " + a.editJob.DisplayID()
		return
	}
	a.editError = ""
	a.editConfirm = true
	a.mode = modeConfirm
}

func validTimeLimit(value string) bool {
	if strings.EqualFold(value, "UNLIMITED") {
		return true
	}
	if value == "" || strings.TrimSpace(value) != value {
		return false
	}
	dayParts := strings.Split(value, "-")
	if len(dayParts) > 2 {
		return false
	}
	if len(dayParts) == 2 {
		if _, err := strconv.ParseUint(dayParts[0], 10, 64); err != nil {
			return false
		}
	}
	timeParts := strings.Split(dayParts[len(dayParts)-1], ":")
	if len(timeParts) < 1 || len(timeParts) > 3 {
		return false
	}
	values := make([]uint64, len(timeParts))
	for i, part := range timeParts {
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return false
		}
		values[i] = n
	}
	if len(dayParts) == 2 && (len(values) < 1 || values[0] > 23) {
		return false
	}
	if len(values) == 3 {
		if values[1] > 59 || values[2] > 59 {
			return false
		}
	} else if len(values) == 2 && values[1] > 59 {
		return false
	}
	return true
}

func showEmpty(value string) string {
	if value == "" {
		return "(empty)"
	}
	return value
}

func (a *App) renderEditForm(out *strings.Builder) {
	fmt.Fprintf(out, "%s\n\nJob: %s  %s\n\n", titleStyle.Render("Edit job"), a.editJob.DisplayID(), a.editJob.Name)
	for i := range a.editInputs {
		fmt.Fprintln(out, a.editInputs[i].View())
	}
	if a.editError != "" {
		fmt.Fprintln(out, errorStyle.Render(a.editError))
	}
	fmt.Fprintln(out, mutedStyle.Render("Enter: next/ review  ↑/↓: change field  Esc: cancel"))
}

func (a *App) renderEditConfirm(out *strings.Builder) {
	fmt.Fprintf(out, "%s\n\nJob: %s  %s\n\n", titleStyle.Render("Confirm job update"), a.editJob.DisplayID(), a.editJob.Name)
	for _, line := range a.editDiff {
		fmt.Fprintln(out, line)
	}
	fmt.Fprintln(out, "\nOnly changed fields will be sent.")
	fmt.Fprintln(out, mutedStyle.Render("Press y to confirm; any other key returns to the form."))
}

func (a *App) runJobEdit() tea.Cmd {
	runner := a.runner
	args := []string{"update", "JobId=" + a.editJob.DisplayID()}
	old := []string{timeLimitValue(a.editJob.TimeLimit), a.editJob.Name, a.editComment}
	fields := []string{"TimeLimit", "Name", "Comment"}
	for i := range fields {
		value := a.editInputs[i].Value()
		if value != old[i] {
			args = append(args, fields[i]+"="+value)
		}
	}
	job := a.editJob
	return func() tea.Msg {
		if runner == nil {
			return editResult{job: job, err: fmt.Errorf("slurm runner is unavailable")}
		}
		_, err := runner.Run(context.Background(), "scontrol", args...)
		return editResult{job: job, err: err}
	}
}

type editResult struct {
	job model.Job
	err error
}
