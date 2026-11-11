package model

import "encoding/json"

// JobDetail contains fields that are fetched only after opening a job.
type JobDetail struct {
	Job       Job
	Requested []TRES
	Allocated []TRES
	// Text forms are used by scontrol, whose schema exposes TRES strings.
	RequestedText string
	AllocatedText string
	Steps         []JobStep
	RawJSON       json.RawMessage
}

type TRES struct {
	ID    int64  `json:"id"`
	Name  string `json:"name"`
	Type  string `json:"type"`
	Count int64  `json:"count"`
}

type JobStep struct {
	ID        string      `json:"id"`
	Name      string      `json:"name"`
	State     []State     `json:"state"`
	ExitCode  ExitCode    `json:"exit_code"`
	Nodes     string      `json:"nodes"`
	Elapsed   int64       `json:"elapsed"`
	Start     NoValNumber `json:"start"`
	End       NoValNumber `json:"end"`
	Limit     NoValNumber `json:"limit"`
	Requested []TRES      `json:"requested"`
	Allocated []TRES      `json:"allocated"`
}
