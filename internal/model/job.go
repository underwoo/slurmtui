package model

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
	SubmitTime       NoValNumber `json:"submit_time"`
	StartTime        NoValNumber `json:"start_time"`
	ArrayJobID       NoValNumber `json:"array_job_id"`
	ArrayTaskID      NoValNumber `json:"array_task_id"`
	ArrayTaskString  string      `json:"array_task_string"`
	HetJobID         NoValNumber `json:"het_job_id"`
	HetJobOffset     NoValNumber `json:"het_job_offset"`
	ExitCode         ExitCode    `json:"exit_code"`
}

type ExitCode struct {
	Status     []string    `json:"status"`
	ReturnCode NoValNumber `json:"return_code"`
	Signal     NoValNumber `json:"signal"`
}
