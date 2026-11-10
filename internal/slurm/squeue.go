package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	"slurmtui/internal/model"
)

type Scope string

const (
	ScopeMe  Scope = "me"
	ScopeAll Scope = "all"
)

type Version struct {
	Major int `json:"major"`
	Minor int `json:"minor"`
	Micro int `json:"micro"`
}

func (v Version) String() string { return fmt.Sprintf("%d.%02d.%d", v.Major, v.Minor, v.Micro) }

func (v Version) Supported() bool {
	return v.Major > 25 || (v.Major == 25 && v.Minor >= 11)
}

func (v *Version) UnmarshalJSON(data []byte) error {
	var text string
	if err := json.Unmarshal(data, &text); err == nil {
		parts := strings.Split(text, ".")
		if len(parts) < 2 || len(parts) > 3 {
			return fmt.Errorf("invalid Slurm version %q", text)
		}
		values := []int{0, 0, 0}
		for i, part := range parts {
			n, err := strconv.Atoi(part)
			if err != nil {
				return fmt.Errorf("invalid Slurm version %q: %w", text, err)
			}
			values[i] = n
		}
		*v = Version{Major: values[0], Minor: values[1], Micro: values[2]}
		return nil
	}
	var object struct {
		Major json.RawMessage `json:"major"`
		Minor json.RawMessage `json:"minor"`
		Micro json.RawMessage `json:"micro"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("decode Slurm version: %w", err)
	}
	var err error
	if v.Major, err = versionPart(object.Major); err != nil {
		return fmt.Errorf("invalid Slurm major version: %w", err)
	}
	if v.Minor, err = versionPart(object.Minor); err != nil {
		return fmt.Errorf("invalid Slurm minor version: %w", err)
	}
	if v.Micro, err = versionPart(object.Micro); err != nil {
		return fmt.Errorf("invalid Slurm micro version: %w", err)
	}
	return nil
}

func versionPart(raw json.RawMessage) (int, error) {
	if len(raw) == 0 || bytes.Equal(raw, []byte("null")) {
		return 0, nil
	}
	var n int
	if err := json.Unmarshal(raw, &n); err == nil {
		return n, nil
	}
	var text string
	if err := json.Unmarshal(raw, &text); err != nil {
		return 0, err
	}
	n, err := strconv.Atoi(text)
	return n, err
}

type ListResult struct {
	Jobs     []model.Job
	Version  Version
	Warnings []string
}

// VersionFromJSON extracts the Slurm version from any Slurm JSON response.
// It is also useful for commands whose job schema differs from squeue.
func VersionFromJSON(data []byte) (Version, error) {
	var envelope struct {
		Meta struct {
			Slurm struct {
				Version Version `json:"version"`
			} `json:"slurm"`
		} `json:"meta"`
	}
	if err := json.Unmarshal(data, &envelope); err != nil {
		return Version{}, fmt.Errorf("decode Slurm version metadata: %w", err)
	}
	if envelope.Meta.Slurm.Version.Major == 0 {
		return Version{}, errors.New("missing meta.slurm.version in Slurm JSON")
	}
	return envelope.Meta.Slurm.Version, nil
}

type queueResponse struct {
	Jobs []model.Job `json:"jobs"`
	Meta struct {
		Slurm struct {
			Version Version `json:"version"`
		} `json:"slurm"`
	} `json:"meta"`
	Errors   []slurmMessage `json:"errors"`
	Warnings []slurmMessage `json:"warnings"`
}

type slurmMessage struct {
	Description string `json:"description"`
	Source      string `json:"source"`
}

func (m slurmMessage) String() string {
	if m.Source == "" {
		return m.Description
	}
	return m.Source + ": " + m.Description
}

// ListJobs loads the queue as JSON. The default scope is the caller's jobs;
// ScopeAll omits --me to request all visible jobs.
func ListJobs(ctx context.Context, runner Runner, scope Scope) (ListResult, error) {
	args := []string{"--json"}
	switch scope {
	case ScopeMe:
		args = append(args, "--me")
	case ScopeAll:
	default:
		return ListResult{}, fmt.Errorf("invalid squeue scope %q", scope)
	}
	data, err := runner.Run(ctx, "squeue", args...)
	if err != nil {
		return ListResult{}, fmt.Errorf("run squeue: %w", err)
	}

	decoder := json.NewDecoder(bytes.NewReader(data))
	var response queueResponse
	if err := decoder.Decode(&response); err != nil {
		return ListResult{}, fmt.Errorf("decode squeue JSON: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err == nil {
			return ListResult{}, errors.New("decode squeue JSON: multiple JSON values")
		}
		return ListResult{}, fmt.Errorf("decode trailing squeue JSON: %w", err)
	}

	version := response.Meta.Slurm.Version
	if version.Major == 0 {
		return ListResult{}, errors.New("squeue JSON is missing meta.slurm.version")
	}
	if !version.Supported() {
		return ListResult{}, fmt.Errorf("slurm %s is unsupported; slurmtui requires Slurm 25.11 or newer", version)
	}
	if len(response.Errors) > 0 {
		return ListResult{}, fmt.Errorf("squeue reported errors: %s", joinMessages(response.Errors))
	}

	warnings := make([]string, 0, len(response.Warnings))
	for _, warning := range response.Warnings {
		if strings.EqualFold(strings.TrimSpace(warning.Description), "Zero jobs to dump") {
			continue
		}
		if value := warning.String(); value != "" {
			warnings = append(warnings, value)
		}
	}
	if response.Jobs == nil {
		response.Jobs = []model.Job{}
	}
	return ListResult{Jobs: response.Jobs, Version: version, Warnings: warnings}, nil
}

func joinMessages(messages []slurmMessage) string {
	parts := make([]string, 0, len(messages))
	for _, message := range messages {
		if value := message.String(); value != "" {
			parts = append(parts, value)
		}
	}
	if len(parts) == 0 {
		return "unspecified error"
	}
	return strings.Join(parts, "; ")
}
