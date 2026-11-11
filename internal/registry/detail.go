package registry

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"slurmtui/internal/model"
)

// DetailSection is the extension seam for job detail panels.
type DetailSection struct {
	ID      string
	Title   string
	Applies func(model.JobDetail) bool
	Render  func(model.JobDetail, time.Time) []string
}

func DetailSections() []DetailSection {
	return []DetailSection{
		{ID: "summary", Title: "Summary", Applies: alwaysDetail, Render: renderSummary},
		{ID: "resources", Title: "Resources", Applies: alwaysDetail, Render: renderResources},
		{ID: "timing", Title: "Timing", Applies: alwaysDetail, Render: renderTiming},
		{ID: "paths", Title: "Paths", Applies: alwaysDetail, Render: renderPaths},
		{ID: "steps", Title: "Steps", Applies: func(d model.JobDetail) bool { return len(d.Steps) > 0 }, Render: renderSteps},
		{ID: "reason", Title: "Reason / Dependency", Applies: alwaysDetail, Render: renderReason},
		{ID: "all", Title: "All fields", Applies: alwaysDetail, Render: renderAllFields},
	}
}

func alwaysDetail(model.JobDetail) bool { return true }

func renderSummary(d model.JobDetail, _ time.Time) []string {
	j := d.Job
	return []string{fmt.Sprintf("Job ID: %s", j.DisplayID()), fmt.Sprintf("Name: %s", emptyDash(j.Name)), fmt.Sprintf("User: %s", emptyDash(j.UserName)), fmt.Sprintf("Account: %s", emptyDash(j.Account)), fmt.Sprintf("Partition: %s", emptyDash(j.Partition)), fmt.Sprintf("QOS: %s", emptyDash(j.QOS)), fmt.Sprintf("State: %s", stateString(j.States)), fmt.Sprintf("Node list: %s", emptyDash(j.NodeList)), fmt.Sprintf("Node count: %s", j.NodeCount.String())}
}

func renderResources(d model.JobDetail, _ time.Time) []string {
	if len(d.Requested) == 0 && len(d.Allocated) == 0 && (d.RequestedText != "" || d.AllocatedText != "") {
		return []string{
			"Requested TRES: " + emptyDash(d.RequestedText),
			"Allocated TRES: " + emptyDash(d.AllocatedText),
		}
	}
	lines := []string{"Requested TRES:"}
	lines = append(lines, renderTRES(d.Requested)...)
	lines = append(lines, "Allocated TRES:")
	lines = append(lines, renderTRES(d.Allocated)...)
	return lines
}

func renderTRES(items []model.TRES) []string {
	if len(items) == 0 {
		return []string{"  —"}
	}
	lines := make([]string, 0, len(items))
	for _, item := range items {
		name := item.Name
		if name == "" {
			name = item.Type
		}
		if name == "" {
			name = fmt.Sprintf("TRES %d", item.ID)
		}
		lines = append(lines, fmt.Sprintf("  %s: %d", name, item.Count))
	}
	return lines
}

func renderTiming(d model.JobDetail, _ time.Time) []string {
	j := d.Job
	return []string{fmt.Sprintf("Submitted: %s", timeString(j.SubmitTime)), fmt.Sprintf("Started: %s", timeString(j.StartTime)), fmt.Sprintf("Ended: %s", timeString(j.EndTime)), fmt.Sprintf("Elapsed: %s", durationString(j.ElapsedSeconds)), fmt.Sprintf("Time limit: %s", j.TimeLimit.String()), fmt.Sprintf("Exit: %s", exitString(j.ExitCode))}
}

func renderPaths(d model.JobDetail, _ time.Time) []string {
	return []string{fmt.Sprintf("Working directory: %s", emptyDash(d.Job.WorkingDirectory)), fmt.Sprintf("Command: %s", emptyDash(d.Job.Command)), fmt.Sprintf("Stdout: %s", emptyDash(d.Job.Stdout)), fmt.Sprintf("Stderr: %s", emptyDash(d.Job.Stderr)), fmt.Sprintf("Stdin: %s", emptyDash(d.Job.Stdin))}
}

func renderSteps(d model.JobDetail, _ time.Time) []string {
	lines := make([]string, 0, len(d.Steps)*4)
	for _, step := range d.Steps {
		lines = append(lines, fmt.Sprintf("%s (%s)", emptyDash(step.ID), emptyDash(step.Name)), fmt.Sprintf("  State: %s", stateString(step.State)), fmt.Sprintf("  Exit: %s  Elapsed: %s", exitString(step.ExitCode), durationString(step.Elapsed)), fmt.Sprintf("  Nodes: %s", emptyDash(step.Nodes)))
	}
	return lines
}

func renderReason(d model.JobDetail, _ time.Time) []string {
	j := d.Job
	return []string{fmt.Sprintf("Reason: %s", emptyDash(j.StateReason)), fmt.Sprintf("Dependency: %s", emptyDash(j.Dependency))}
}

func renderAllFields(d model.JobDetail, _ time.Time) []string {
	if len(d.RawJSON) == 0 {
		return []string{"  (raw fields unavailable)"}
	}
	var pretty bytes.Buffer
	if err := json.Indent(&pretty, d.RawJSON, "", "  "); err != nil {
		return []string{"  (could not format raw fields)"}
	}
	return strings.Split(pretty.String(), "\n")
}

func timeString(value model.NoValNumber) string {
	if n, ok := value.Value(); ok {
		return time.Unix(n, 0).UTC().Format("2006-01-02 15:04:05 UTC")
	}
	return value.String()
}

func durationString(seconds int64) string {
	if seconds < 0 {
		return "—"
	}
	return (time.Duration(seconds) * time.Second).String()
}

func exitString(value model.ExitCode) string {
	parts := make([]string, 0, 2)
	if code, ok := value.ReturnCode.Value(); ok {
		parts = append(parts, fmt.Sprintf("code %d", code))
	}
	if value.Signal.Name != "" {
		parts = append(parts, "signal "+value.Signal.Name)
	}
	if len(parts) == 0 && len(value.Status) > 0 {
		return strings.Join(value.Status, ", ")
	}
	if len(parts) == 0 {
		return "—"
	}
	return strings.Join(parts, ", ")
}

func stateString(states []model.State) string {
	if len(states) == 0 {
		return "—"
	}
	values := make([]string, len(states))
	for i, state := range states {
		values[i] = string(state)
	}
	return strings.Join(values, ", ")
}

func emptyDash(value string) string {
	if value == "" {
		return "—"
	}
	return value
}
