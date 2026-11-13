package ui

import (
	"regexp"
	"time"
	"unicode"

	"slurmtui/internal/model"
	"slurmtui/internal/registry"
)

func compileFilter(pattern string) (*regexp.Regexp, error) {
	if pattern == "" {
		return nil, nil
	}
	for _, r := range pattern {
		if unicode.IsUpper(r) {
			return regexp.Compile(pattern)
		}
	}
	return regexp.Compile("(?i)" + pattern)
}

func filterJobs(jobs []model.Job, columns []registry.Column, pattern *regexp.Regexp, now time.Time, location *time.Location) []model.Job {
	if pattern == nil {
		return jobs
	}
	filtered := make([]model.Job, 0, len(jobs))
	for _, job := range jobs {
		for _, column := range columns {
			if pattern.MatchString(column.Value(job, now, location)) {
				filtered = append(filtered, job)
				break
			}
		}
	}
	return filtered
}
