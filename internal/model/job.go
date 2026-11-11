package model

import (
	"fmt"
	"strconv"
)

// State is one of Slurm's job-state names. It remains a string so newer
// Slurm states can be displayed even before this package names them.
type State string

const (
	StateBootFail    State = "BOOT_FAIL"
	StateCancelled   State = "CANCELLED"
	StateCompleted   State = "COMPLETED"
	StateCompleting  State = "COMPLETING"
	StateConfiguring State = "CONFIGURING"
	StateDeadline    State = "DEADLINE"
	StateFailed      State = "FAILED"
	StateNodeFail    State = "NODE_FAIL"
	StateOutOfMemory State = "OUT_OF_MEMORY"
	StatePending     State = "PENDING"
	StatePreempted   State = "PREEMPTED"
	StateRequeued    State = "REQUEUED"
	StateResizing    State = "RESIZING"
	StateRunning     State = "RUNNING"
	StateSuspended   State = "SUSPENDED"
	StateTimeout     State = "TIMEOUT"
)

// Job contains the fields needed by the queue and selected-job views.
// Unknown fields in Slurm's JSON are intentionally ignored.
type Job struct {
	ID               int64       `json:"job_id"`
	Name             string      `json:"name"`
	UserName         string      `json:"user_name"`
	UserID           int64       `json:"user_id"`
	GroupName        string      `json:"group_name"`
	GroupID          int64       `json:"group_id"`
	Account          string      `json:"account"`
	Partition        string      `json:"partition"`
	QOS              string      `json:"qos"`
	States           []State     `json:"job_state"`
	StateDescription string      `json:"state_description"`
	StateReason      string      `json:"state_reason"`
	Dependency       string      `json:"dependency"`
	NodeList         string      `json:"nodes"`
	ScheduledNodes   string      `json:"scheduled_nodes"`
	NodeCount        NoValNumber `json:"node_count"`
	CPUs             NoValNumber `json:"cpus"`
	Tasks            NoValNumber `json:"tasks"`
	Priority         NoValNumber `json:"priority"`
	TimeLimit        NoValNumber `json:"time_limit"`
	ElapsedSeconds   int64       `json:"elapsed_seconds"`
	SubmitTime       NoValNumber `json:"submit_time"`
	StartTime        NoValNumber `json:"start_time"`
	EndTime          NoValNumber `json:"end_time"`
	ArrayJobID       NoValNumber `json:"array_job_id"`
	ArrayTaskID      NoValNumber `json:"array_task_id"`
	ArrayTaskString  string      `json:"array_task_string"`
	HetJobID         NoValNumber `json:"het_job_id"`
	HetJobOffset     NoValNumber `json:"het_job_offset"`
	ExitCode         ExitCode    `json:"exit_code"`
	WorkingDirectory string      `json:"current_working_directory"`
	Command          string      `json:"command"`
	Stdout           string      `json:"stdout_expanded"`
	Stderr           string      `json:"stderr_expanded"`
	Stdin            string      `json:"stdin_expanded"`
	TRESRequested    string      `json:"tres_req_str"`
	TRESAllocated    string      `json:"tres_alloc_str"`
}

// DisplayID returns the Slurm identifier for an array task or heterogeneous
// job component, falling back to the numeric job ID.
func (j Job) DisplayID() string {
	if j.ArrayJobID.Set && j.ArrayTaskID.Set && j.ArrayTaskID.Number > 0 {
		return fmt.Sprintf("%d_%d", j.ArrayJobID.Number, j.ArrayTaskID.Number)
	}
	if j.HetJobID.Set && j.HetJobOffset.Set && j.HetJobOffset.Number > 0 {
		return fmt.Sprintf("%d+%d", j.HetJobID.Number, j.HetJobOffset.Number)
	}
	return strconv.FormatInt(j.ID, 10)
}

type ExitCode struct {
	Status     []string    `json:"status"`
	ReturnCode NoValNumber `json:"return_code"`
	Signal     ExitSignal  `json:"signal"`
}

// ExitSignal describes the terminating signal reported by Slurm.
type ExitSignal struct {
	ID   NoValNumber `json:"id"`
	Name string      `json:"name"`
}
