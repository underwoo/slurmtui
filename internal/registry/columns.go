// Package registry contains the built-in job columns shared by the UI.
package registry

import (
	"fmt"
	"strings"
	"time"

	"slurmtui/internal/model"
)

type Tab string

const (
	QueueTab  Tab = "queue"
	RecentTab Tab = "recent"
)

type Column struct {
	ID             string
	Header         string
	Width          int
	MinWidth       int
	DefaultVisible bool
	RecentOnly     bool
	Value          func(model.Job, time.Time, *time.Location) string
	Less           func(model.Job, model.Job) bool
}

func DefaultColumns() []Column {
	columns := []Column{
		{ID: "job_id", Header: "JobID", Width: 12, MinWidth: 6, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.DisplayID() }},
		{ID: "cluster", Header: "Cluster", Width: 10, MinWidth: 6, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.Cluster }},
		{ID: "name", Header: "Name", Width: 22, MinWidth: 7, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.Name }},
		{ID: "user", Header: "User", Width: 12, MinWidth: 6, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.UserName }},
		{ID: "account", Header: "Account", Width: 12, MinWidth: 6, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.Account }},
		{ID: "partition", Header: "Partition", Width: 10, MinWidth: 6, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.Partition }},
		{ID: "qos", Header: "QOS", Width: 9, MinWidth: 5, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.QOS }},
		{ID: "state", Header: "State", Width: 14, MinWidth: 7, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return firstState(job.States) }},
		{ID: "elapsed_limit", Header: "Elapsed/Limit", Width: 19, MinWidth: 10, DefaultVisible: true, Value: elapsedLimit},
		{ID: "nodes", Header: "Nodes", Width: 7, MinWidth: 5, DefaultVisible: true, Value: func(job model.Job, _ time.Time, _ *time.Location) string { return job.NodeCount.String() }},
		{ID: "node_reason", Header: "NodeList(Reason)", Width: 24, MinWidth: 8, DefaultVisible: true, Value: nodeListReason},
		{ID: "end", Header: "End", Width: 16, MinWidth: 8, DefaultVisible: true, RecentOnly: true, Value: func(job model.Job, _ time.Time, location *time.Location) string {
			if value, ok := job.EndTime.Value(); ok {
				return time.Unix(value, 0).In(localLocation(location)).Format("01-02 15:04")
			}
			return "—"
		}},
	}
	for i := range columns {
		columns[i].Less = lessForColumn(columns[i].ID)
	}
	return columns
}

func firstState(states []model.State) string {
	if len(states) == 0 {
		return "UNKNOWN"
	}
	return string(states[0])
}

func elapsedLimit(job model.Job, now time.Time, _ *time.Location) string {
	state := firstState(job.States)
	elapsed := "—"
	if state != string(model.StatePending) {
		seconds := job.ElapsedSeconds
		if state == string(model.StateRunning) {
			if started, ok := job.StartTime.Value(); ok && now.Unix() >= started {
				seconds = now.Unix() - started
			}
		}
		elapsed = formatDuration(seconds)
	}
	limit := "—"
	if job.TimeLimit.Infinite {
		limit = "∞"
	} else if value, ok := job.TimeLimit.Value(); ok {
		limit = formatDuration(value * 60)
	}
	return elapsed + "/" + limit
}

func formatDuration(seconds int64) string {
	if seconds < 0 {
		seconds = 0
	}
	hours := seconds / 3600
	minutes := (seconds % 3600) / 60
	remaining := seconds % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, remaining)
	}
	return fmt.Sprintf("%d:%02d", minutes, remaining)
}

func nodeListReason(job model.Job, _ time.Time, _ *time.Location) string {
	state := firstState(job.States)
	if state == string(model.StateRunning) && job.NodeList != "" {
		return job.NodeList
	}
	if state == string(model.StatePending) {
		reason := job.StateReason
		if reason == "" {
			reason = job.StateDescription
		}
		if reason != "" {
			return "(" + reason + ")"
		}
	}
	if job.NodeList != "" {
		return job.NodeList
	}
	if job.StateReason != "" && !strings.EqualFold(job.StateReason, "none") {
		return "(" + job.StateReason + ")"
	}
	return "—"
}

func localLocation(location *time.Location) *time.Location {
	if location == nil {
		return time.Local
	}
	return location
}

func VisibleColumns(tab Tab) []Column {
	all := AllColumns(tab)
	visible := make([]Column, 0, len(all))
	for _, column := range all {
		if column.DefaultVisible {
			visible = append(visible, column)
		}
	}
	return visible
}

// AllColumns returns every built-in column supported by a tab, including
// columns that are hidden by default but can be selected by the user.
func AllColumns(tab Tab) []Column {
	all := DefaultColumns()
	columns := make([]Column, 0, len(all))
	for _, column := range all {
		if !column.RecentOnly || tab == RecentTab {
			columns = append(columns, column)
		}
	}
	return columns
}

// ColumnsForIDs resolves an optional configured order. Unknown IDs are omitted
// and returned so the UI can surface them to the user.
func ColumnsForIDs(tab Tab, ids []string) ([]Column, []string) {
	if ids == nil {
		return VisibleColumns(tab), nil
	}
	available := make(map[string]Column)
	for _, column := range AllColumns(tab) {
		available[column.ID] = column
	}
	columns := make([]Column, 0, len(ids))
	unknown := make([]string, 0)
	seen := make(map[string]bool)
	for _, id := range ids {
		column, ok := available[id]
		if !ok {
			unknown = append(unknown, id)
			continue
		}
		if seen[id] {
			continue
		}
		seen[id] = true
		columns = append(columns, column)
	}
	return columns, unknown
}
