// Package service combines Slurm data sources into views used by the UI.
package service

import (
	"strconv"

	"slurmtui/internal/model"
)

// Merge returns queue jobs first, then recent jobs not already present.
// Current squeue records win when both sources report the same job.
func Merge(queue, recent []model.Job) []model.Job {
	merged := make([]model.Job, 0, len(queue)+len(recent))
	seen := make(map[string]struct{}, len(queue)+len(recent))
	appendUnique := func(job model.Job) {
		key := job.DisplayID()
		if key == "" || key == "0" {
			key = strconv.FormatInt(job.ID, 10)
		}
		if _, ok := seen[key]; ok {
			return
		}
		seen[key] = struct{}{}
		merged = append(merged, job)
	}
	for _, job := range queue {
		appendUnique(job)
	}
	for _, job := range recent {
		appendUnique(job)
	}
	return merged
}
