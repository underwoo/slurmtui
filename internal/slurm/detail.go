package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"slurmtui/internal/model"
)

// GetJobDetail fetches detail from the source appropriate for the selected
// row. If scontrol lost a job between the queue refresh and open, sacct is used.
func GetJobDetail(ctx context.Context, runner Runner, job model.Job, fromRecent bool) (model.JobDetail, error) {
	if runner == nil {
		return model.JobDetail{}, errors.New("slurm runner is unavailable")
	}
	if fromRecent {
		return getSacctDetail(ctx, runner, job.DisplayID())
	}
	data, err := runner.Run(ctx, "scontrol", "show", "job", job.DisplayID(), "--json")
	if err != nil {
		if isInvalidJobID(err) {
			return getSacctDetail(ctx, runner, job.DisplayID())
		}
		return model.JobDetail{}, fmt.Errorf("run scontrol: %w", err)
	}
	detail, err := decodeScontrolDetail(data)
	if err != nil && isInvalidJobID(err) {
		return getSacctDetail(ctx, runner, job.DisplayID())
	}
	return detail, err
}

func isInvalidJobID(err error) bool {
	text := strings.ToLower(err.Error())
	return strings.Contains(text, "invalid job id") || strings.Contains(text, "invalid jobid") || strings.Contains(text, "job id not found")
}

func decodeScontrolDetail(data []byte) (model.JobDetail, error) {
	var envelope struct {
		Jobs   []json.RawMessage `json:"jobs"`
		Errors []slurmMessage    `json:"errors"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return model.JobDetail{}, fmt.Errorf("decode scontrol JSON: %w", err)
	}
	if len(envelope.Errors) > 0 {
		return model.JobDetail{}, fmt.Errorf("scontrol reported errors: %s", joinMessages(envelope.Errors))
	}
	if len(envelope.Jobs) == 0 {
		return model.JobDetail{}, errors.New("scontrol JSON contains no job")
	}
	var job model.Job
	if err := json.Unmarshal(envelope.Jobs[0], &job); err != nil {
		return model.JobDetail{}, fmt.Errorf("decode scontrol job: %w", err)
	}
	return model.JobDetail{
		Job:           job,
		RequestedText: job.TRESRequested,
		AllocatedText: job.TRESAllocated,
		RawJSON:       append(json.RawMessage(nil), envelope.Jobs[0]...),
	}, nil
}

func getSacctDetail(ctx context.Context, runner Runner, id string) (model.JobDetail, error) {
	data, err := runner.Run(ctx, "sacct", "--json", "-j", id)
	if err != nil {
		return model.JobDetail{}, fmt.Errorf("run sacct: %w", err)
	}
	return decodeSacctDetail(data, id)
}

func decodeSacctDetail(data []byte, id string) (model.JobDetail, error) {
	decoder := json.NewDecoder(bytes.NewReader(data))
	var envelope struct {
		Jobs []sacctDetailJob `json:"jobs"`
		Meta struct {
			Slurm struct {
				Version Version `json:"version"`
			} `json:"slurm"`
		} `json:"meta"`
		Errors []slurmMessage `json:"errors"`
	}
	if err := decoder.Decode(&envelope); err != nil {
		return model.JobDetail{}, fmt.Errorf("decode sacct JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		return model.JobDetail{}, errors.New("decode sacct JSON: multiple or invalid trailing JSON")
	}
	if len(envelope.Errors) > 0 {
		return model.JobDetail{}, fmt.Errorf("sacct reported errors: %s", joinMessages(envelope.Errors))
	}
	for _, record := range envelope.Jobs {
		job := record.job()
		if job.DisplayID() == id || fmt.Sprint(record.ID) == id {
			job.WorkingDirectory, job.Stdout, job.Stderr, job.Stdin = record.WorkingDirectory, record.Stdout, record.Stderr, record.Stdin
			return model.JobDetail{Job: job, Requested: record.TRES.Requested, Allocated: record.TRES.Allocated, Steps: record.jobSteps(), RawJSON: append(json.RawMessage(nil), record.raw...)}, nil
		}
	}
	return model.JobDetail{}, fmt.Errorf("sacct returned no detail for job %s", id)
}

type sacctDetailJob struct {
	sacctJob
	TRES struct {
		Requested []model.TRES `json:"requested"`
		Allocated []model.TRES `json:"allocated"`
	} `json:"tres"`
	Steps []sacctStep `json:"steps"`
	raw   json.RawMessage
}

func (record *sacctDetailJob) UnmarshalJSON(data []byte) error {
	type alias sacctDetailJob
	var decoded alias
	if err := json.Unmarshal(data, &decoded); err != nil {
		return err
	}
	*record = sacctDetailJob(decoded)
	record.raw = append(json.RawMessage(nil), data...)
	return nil
}

type sacctStep struct {
	Step struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"step"`
	State    []model.State  `json:"state"`
	ExitCode model.ExitCode `json:"exit_code"`
	Nodes    struct {
		Range string `json:"range"`
	} `json:"nodes"`
	Time struct {
		Elapsed int64             `json:"elapsed"`
		Start   model.NoValNumber `json:"start"`
		End     model.NoValNumber `json:"end"`
		Limit   model.NoValNumber `json:"limit"`
	} `json:"time"`
	TRES struct {
		Requested TRESList `json:"requested"`
		Allocated TRESList `json:"allocated"`
	} `json:"tres"`
}

func (step sacctStep) model() model.JobStep {
	return model.JobStep{ID: step.Step.ID, Name: step.Step.Name, State: step.State, ExitCode: step.ExitCode, Nodes: step.Nodes.Range, Elapsed: step.Time.Elapsed, Start: step.Time.Start, End: step.Time.End, Limit: step.Time.Limit, Requested: []model.TRES(step.TRES.Requested), Allocated: []model.TRES(step.TRES.Allocated)}
}

// TRESList accepts both job-level arrays and step-level metric objects.
// For the latter, total is the most useful stable value for display.
type TRESList []model.TRES

func (items *TRESList) UnmarshalJSON(data []byte) error {
	var list []model.TRES
	if err := json.Unmarshal(data, &list); err == nil {
		*items = list
		return nil
	}
	var metrics struct {
		Total   []model.TRES `json:"total"`
		Average []model.TRES `json:"average"`
	}
	if err := json.Unmarshal(data, &metrics); err != nil {
		return err
	}
	if metrics.Total != nil {
		*items = metrics.Total
	} else {
		*items = metrics.Average
	}
	return nil
}

func (record sacctDetailJob) jobSteps() []model.JobStep {
	steps := make([]model.JobStep, 0, len(record.Steps))
	for _, step := range record.Steps {
		steps = append(steps, step.model())
	}
	return steps
}
