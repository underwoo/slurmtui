// Command scrubfixtures creates deterministic, anonymized Slurm JSON fixtures.
// Raw cluster dumps must stay outside the repository.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
)

func main() {
	input := flag.String("input", "", "raw Slurm JSON input file")
	output := flag.String("output", "", "anonymized JSON output file")
	maxJobs := flag.Int("max-jobs", 0, "maximum number of jobs to retain (0 keeps all)")
	offset := flag.Int("offset", 0, "number of selected jobs to skip before writing")
	count := flag.Int("count", 0, "maximum jobs to write (0 writes all remaining jobs)")
	compact := flag.Bool("compact", false, "write compact JSON")
	omitJobResources := flag.Bool("omit-job-resources", false, "omit the large job_resources field from jobs")
	flag.Parse()
	if *input == "" || *output == "" {
		fmt.Fprintln(os.Stderr, "usage: scrubfixtures -input raw.json -output testdata/fixture.json [-max-jobs N]")
		os.Exit(2)
	}
	if *maxJobs < 0 || *offset < 0 || *count < 0 {
		fmt.Fprintln(os.Stderr, "max-jobs, offset, and count must be zero or greater")
		os.Exit(2)
	}
	if err := scrubFile(*input, *output, *maxJobs, *offset, *count, *compact, *omitJobResources); err != nil {
		fmt.Fprintln(os.Stderr, "scrubfixtures:", err)
		os.Exit(1)
	}
}

func scrubFile(inputPath, outputPath string, maxJobs, offset, count int, compact, omitJobResources bool) error {
	input, err := os.Open(inputPath)
	if err != nil {
		return err
	}
	defer func() { _ = input.Close() }()
	decoder := json.NewDecoder(input)
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return fmt.Errorf("decode input: %w", err)
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("input contains multiple JSON values")
		}
		return fmt.Errorf("decode trailing input: %w", err)
	}
	cleaner := newScrubber()
	value = cleaner.cleanRoot(value, maxJobs)
	if omitJobResources {
		value = withoutJobResources(value)
	}
	value = sliceJobs(value, offset, count)
	if err := os.MkdirAll(dirName(outputPath), 0o755); err != nil {
		return err
	}
	output, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	encoder := json.NewEncoder(output)
	if !compact {
		encoder.SetIndent("", "  ")
	}
	encodeErr := encoder.Encode(value)
	closeErr := output.Close()
	if encodeErr != nil {
		return encodeErr
	}
	return closeErr
}

func withoutJobResources(value any) any {
	root, ok := value.(map[string]any)
	if !ok {
		return value
	}
	jobs, ok := root["jobs"].([]any)
	if !ok {
		return value
	}
	for _, raw := range jobs {
		if job, ok := raw.(map[string]any); ok {
			delete(job, "job_resources")
		}
	}
	return root
}

func dirName(path string) string {
	if i := strings.LastIndexByte(path, '/'); i >= 0 {
		if i == 0 {
			return "/"
		}
		return path[:i]
	}
	return "."
}

type scrubber struct {
	maps map[string]map[string]string
	seq  map[string]int
}

func newScrubber() *scrubber {
	return &scrubber{maps: make(map[string]map[string]string), seq: make(map[string]int)}
}

func (s *scrubber) cleanRoot(value any, maxJobs int) any {
	root, ok := value.(map[string]any)
	if !ok {
		return s.clean("", value)
	}
	if jobs, ok := root["jobs"].([]any); ok && maxJobs > 0 && len(jobs) > maxJobs {
		root["jobs"] = prioritizePending(jobs, maxJobs)
	}
	return s.cleanMap(root)
}

// Put pending examples first when trimming, then fill the limit in input order.
func prioritizePending(jobs []any, limit int) []any {
	pendingJobs := make([]any, 0, limit)
	others := make([]any, 0, len(jobs))
	for _, raw := range jobs {
		job, _ := raw.(map[string]any)
		states, _ := job["job_state"].([]any)
		isPending := false
		for _, state := range states {
			if strings.EqualFold(fmt.Sprint(state), "PENDING") {
				isPending = true
				break
			}
		}
		if isPending {
			pendingJobs = append(pendingJobs, raw)
		} else {
			others = append(others, raw)
		}
	}
	selected := append(pendingJobs, others...)
	if len(selected) > limit {
		selected = selected[:limit]
	}
	return selected
}

func sliceJobs(value any, offset, count int) any {
	root, ok := value.(map[string]any)
	if !ok {
		return value
	}
	jobs, ok := root["jobs"].([]any)
	if !ok {
		return value
	}
	if offset >= len(jobs) {
		root["jobs"] = []any{}
		return root
	}
	end := len(jobs)
	if count > 0 && offset+count < end {
		end = offset + count
	}
	root["jobs"] = jobs[offset:end]
	return root
}

func (s *scrubber) clean(key string, value any) any {
	return s.cleanAt(key, "", value)
}

func (s *scrubber) cleanAt(key, parent string, value any) any {
	if value == nil {
		return nil
	}
	normalized := strings.ToLower(key)
	switch normalized {
	case "comment", "admin_comment", "system_comment", "job_comment", "extra":
		return scrubStrings(value, "")
	case "script":
		if _, ok := value.(string); ok {
			return "#!/bin/sh\n# scrubbed job script\n"
		}
		return scrubStrings(value, "[scrubbed script]")
	}
	if replacement, ok := s.replacement(normalized, parent, value); ok {
		return replacement
	}
	switch value := value.(type) {
	case map[string]any:
		return s.cleanMap(value, normalized)
	case []any:
		cleaned := make([]any, len(value))
		for i, item := range value {
			cleaned[i] = s.cleanAt(key, parent, item)
		}
		return cleaned
	default:
		return value
	}
}

func (s *scrubber) cleanMap(value map[string]any, parent ...string) map[string]any {
	parentKey := ""
	if len(parent) != 0 {
		parentKey = parent[0]
	}
	cleaned := make(map[string]any, len(value))
	keys := make([]string, 0, len(value))
	for key := range value {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		item := value[key]
		cleaned[key] = s.cleanAt(key, parentKey, item)
	}
	return cleaned
}

func scrubStrings(value any, replacement string) any {
	switch value := value.(type) {
	case string:
		return replacement
	case map[string]any:
		cleaned := make(map[string]any, len(value))
		for key, item := range value {
			cleaned[key] = scrubStrings(item, replacement)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(value))
		for i, item := range value {
			cleaned[i] = scrubStrings(item, replacement)
		}
		return cleaned
	default:
		return value
	}
}

func (s *scrubber) replacement(key, parent string, value any) (any, bool) {
	if key == "user_id" || key == "group_id" || key == "association_id" || (key == "id" && parent == "association") {
		base := 10000
		switch key {
		case "group_id":
			base = 20000
		case "association_id":
			base = 30000
		}
		return s.fakeNumber(key, fmt.Sprint(value), base), true
	}
	text, isText := value.(string)
	if !isText {
		return nil, false
	}
	switch key {
	case "user_name", "user", "mail_user", "username", "kill_request_user":
		return s.fake("user", text, "user"), true
	case "group_name", "group":
		return s.fake("group", text, "group"), true
	case "account":
		return s.fake("account", text, "account"), true
	case "association_id":
		return s.fakeNumber("association_id", text, 30000), true
	case "wckey":
		return s.fake(key, text, "wckey"), true
	case "name":
		if parent != "jobs" && parent != "reservation" {
			return nil, false
		}
		return s.fake(key, text, "job"), true
	case "partition":
		return s.fake(key, text, "partition"), true
	case "qos", "qosreq":
		return s.fake(key, text, "qos"), true
	case "cluster", "federation_origin":
		return s.fake("cluster", text, "cluster"), true
	case "current_working_directory", "working_directory", "cwd", "stdin", "stdout", "stderr", "standard_input", "standard_output", "standard_error", "stdin_expanded", "stdout_expanded", "stderr_expanded":
		return s.fakePath(key, text), true
	case "command", "submit_line":
		return "./job.sh", true
	case "nodes", "node_list", "list", "node_name", "required_nodes", "scheduled_nodes", "allocating_node", "batch_host", "failed_node", "allocation_nodes":
		return s.fake("node", text, "node"), true
	}
	return nil, false
}

func (s *scrubber) fake(kind, original, prefix string) string {
	if original == "" {
		return original
	}
	if s.maps[kind] == nil {
		s.maps[kind] = make(map[string]string)
	}
	if value, ok := s.maps[kind][original]; ok {
		return value
	}
	s.seq[kind]++
	value := fmt.Sprintf("%s-%02d", prefix, s.seq[kind])
	s.maps[kind][original] = value
	return value
}

func (s *scrubber) fakeNumber(kind, original string, base int) any {
	if s.maps[kind] == nil {
		s.maps[kind] = make(map[string]string)
	}
	if value, ok := s.maps[kind][original]; ok {
		n, _ := strconv.Atoi(value)
		return n
	}
	s.seq[kind]++
	n := base + s.seq[kind]
	s.maps[kind][original] = strconv.Itoa(n)
	return n
}

func (s *scrubber) fakePath(key, original string) string {
	if original == "" {
		return ""
	}
	return "/tmp/slurmtui-fixture/" + s.fake("path", original, "path")
}
