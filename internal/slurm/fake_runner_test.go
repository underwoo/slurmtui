package slurm

import (
	"context"
	"fmt"
	"strings"
)

// FakeRunner replays recorded command fixtures by executable and argv.
type FakeRunner struct {
	responses map[string][]byte
	errors    map[string]error
	calls     []string
}

func newFakeRunner() *FakeRunner {
	return &FakeRunner{responses: make(map[string][]byte), errors: make(map[string]error)}
}

func (r *FakeRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	key := commandKey(name, args)
	r.calls = append(r.calls, key)
	if err := r.errors[key]; err != nil {
		return nil, err
	}
	data, ok := r.responses[key]
	if !ok {
		return nil, fmt.Errorf("no fake response for %s", key)
	}
	return data, nil
}

func commandKey(name string, args []string) string {
	return name + " " + strings.Join(args, " ")
}
