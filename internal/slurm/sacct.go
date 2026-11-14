package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"slurmtui/internal/model"
)

// ListRecent loads terminal jobs that have been active within window. Active
// jobs are taken from squeue and are intentionally excluded here.
func ListRecent(ctx context.Context, runner Runner, scope Scope, window time.Duration, now func() time.Time) (ListResult, error) {
	if scope != ScopeMe && scope != ScopeAll {
		return ListResult{}, fmt.Errorf("invalid sacct scope %q", scope)
	}
	if window <= 0 {
		return ListResult{}, fmt.Errorf("recent window must be positive")
	}
	if now == nil {
		now = time.Now
	}
	// sacct interprets a timezone-free -S value in its local timezone. Keep
	// the injected clock's location so tests and callers can control that zone.
	since := now().Add(-window).Format("2006-01-02T15:04:05")
	args := []string{"--json", "-X", "-S", since}
	if scope == ScopeAll {
		args = append(args, "-a")
	}
	data, err := runner.Run(ctx, "sacct", args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("run sacct: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	var response sacctResponse
	if err := decoder.Decode(&response); err != nil {
		return ListResult{}, fmt.Errorf("decode sacct JSON: %w", err)
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		return ListResult{}, errors.New("decode sacct JSON: multiple or invalid trailing JSON")
	}
	version := response.Meta.Slurm.Version
	if version.Major == 0 {
		return ListResult{}, errors.New("sacct JSON is missing meta.slurm.version")
	}
	if !version.Supported() {
		return ListResult{}, fmt.Errorf("slurm %s is unsupported; slurmtui requires Slurm 25.11 or newer", version)
	}
	if len(response.Errors) > 0 {
		return ListResult{}, fmt.Errorf("sacct reported errors: %s", joinMessages(response.Errors))
	}
	warnings := make([]string, 0, len(response.Warnings))
	for _, warning := range response.Warnings {
		if value := warning.String(); value != "" {
			warnings = append(warnings, value)
		}
	}
	jobs := make([]model.Job, 0, len(response.Jobs))
	for _, record := range response.Jobs {
		job := record.job()
		if isTerminal(job.States) {
			jobs = append(jobs, job)
		}
	}
	return ListResult{Jobs: jobs, Version: version, Warnings: warnings}, nil
}

type sacctResponse struct {
	Jobs []sacctJob `json:"jobs"`
	Meta struct {
		Slurm struct {
			Version Version `json:"version"`
		} `json:"slurm"`
	} `json:"meta"`
	Errors   []slurmMessage `json:"errors"`
	Warnings []slurmMessage `json:"warnings"`
}

type sacctJob struct {
	ID        int64  `json:"job_id"`
	Name      string `json:"name"`
	User      string `json:"user"`
	UserID    int64  `json:"user_id"`
	Group     string `json:"group"`
	GroupID   int64  `json:"group_id"`
	Account   string `json:"account"`
	Cluster   string `json:"cluster"`
	Partition string `json:"partition"`
	QOS       string `json:"qos"`
	State     struct {
		Current []model.State `json:"current"`
		Reason  string        `json:"reason"`
	} `json:"state"`
	Dependency      string `json:"dependency"`
	Nodes           string `json:"nodes"`
	AllocationNodes int64  `json:"allocation_nodes"`
	Time            struct {
		Elapsed    int64             `json:"elapsed"`
		Start      int64             `json:"start"`
		End        int64             `json:"end"`
		Submission int64             `json:"submission"`
		Limit      model.NoValNumber `json:"limit"`
	} `json:"time"`
	Array struct {
		JobID  int64             `json:"job_id"`
		TaskID model.NoValNumber `json:"task_id"`
		Task   string            `json:"task"`
	} `json:"array"`
	Het struct {
		JobID  int64             `json:"job_id"`
		Offset model.NoValNumber `json:"job_offset"`
	} `json:"het"`
	ExitCode         model.ExitCode `json:"exit_code"`
	WorkingDirectory string         `json:"working_directory"`
	Command          string         `json:"script"`
	SubmitLine       string         `json:"submit_line"`
	Stdout           string         `json:"stdout"`
	Stderr           string         `json:"stderr"`
	Stdin            string         `json:"stdin"`
}

func (record sacctJob) job() model.Job {
	job := model.Job{
		ID: record.ID, Name: record.Name, UserName: record.User,
		UserID: record.UserID, GroupName: record.Group, GroupID: record.GroupID,
		Account: record.Account, Cluster: record.Cluster, Partition: record.Partition, QOS: record.QOS,
		States: record.State.Current, StateReason: record.State.Reason,
		Dependency: record.Dependency, NodeList: record.Nodes,
		NodeCount: present(record.AllocationNodes),
		TimeLimit: record.Time.Limit, ElapsedSeconds: record.Time.Elapsed,
		SubmitTime: present(record.Time.Submission), StartTime: present(record.Time.Start),
		EndTime:          present(record.Time.End),
		ExitCode:         record.ExitCode,
		WorkingDirectory: record.WorkingDirectory, Command: firstNonEmpty(record.SubmitLine, record.Command),
		Stdout: record.Stdout, Stderr: record.Stderr, Stdin: record.Stdin,
	}
	if record.Array.JobID > 0 {
		job.ArrayJobID = present(record.Array.JobID)
		job.ArrayTaskID = record.Array.TaskID
		job.ArrayTaskString = record.Array.Task
	}
	if record.Het.JobID > 0 {
		job.HetJobID = present(record.Het.JobID)
		job.HetJobOffset = record.Het.Offset
	}
	return job
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}

func present(value int64) model.NoValNumber {
	return model.NoValNumber{Set: value > 0, Number: value}
}

func isTerminal(states []model.State) bool {
	for _, state := range states {
		switch state {
		case model.StateCompleted, model.StateFailed, model.StateCancelled,
			model.StateTimeout, model.StateOutOfMemory, model.StateNodeFail,
			model.StatePreempted, model.StateBootFail, model.StateDeadline:
			return true
		}
	}
	return false
}
