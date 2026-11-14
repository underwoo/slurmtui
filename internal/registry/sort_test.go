package registry

import (
	"sort"
	"testing"

	"slurmtui/internal/model"
)

func TestColumnComparators(t *testing.T) {
	low := model.Job{
		ID: 2, Name: "a", Cluster: "a", UserName: "a", Account: "a", Partition: "a", QOS: "a",
		States: []model.State{model.StateRunning}, ElapsedSeconds: 2,
		TimeLimit: model.NoValNumber{Set: true, Number: 2}, NodeCount: model.NoValNumber{Set: true, Number: 2},
		NodeList: "a", EndTime: model.NoValNumber{Set: true, Number: 2},
	}
	high := model.Job{
		ID: 10, Name: "b", Cluster: "b", UserName: "b", Account: "b", Partition: "b", QOS: "b",
		States: []model.State{model.StateRunning}, ElapsedSeconds: 10,
		TimeLimit: model.NoValNumber{Set: true, Number: 10}, NodeCount: model.NoValNumber{Set: true, Number: 10},
		NodeList: "b", EndTime: model.NoValNumber{Set: true, Number: 10},
	}
	for _, column := range DefaultColumns() {
		t.Run(column.ID, func(t *testing.T) {
			if column.Less == nil {
				t.Fatal("column has no comparator")
			}
			a, b := low, high
			if column.ID == "state" {
				a.States = []model.State{model.StateCompleted}
				b.States = []model.State{model.StateFailed}
			}
			if !column.Less(a, b) || column.Less(b, a) {
				t.Fatalf("comparator did not order lower %s before higher value", column.ID)
			}
		})
	}
}

func TestColumnComparatorsPlaceUnsetValuesLast(t *testing.T) {
	valid := model.Job{
		ID: 10, Name: "job", Cluster: "cluster-01", UserName: "user", Account: "account", Partition: "partition", QOS: "normal",
		States: []model.State{model.StateRunning}, ElapsedSeconds: 10,
		TimeLimit: model.NoValNumber{Set: true, Number: 60}, NodeCount: model.NoValNumber{Set: true, Number: 2},
		NodeList: "node-01", EndTime: model.NoValNumber{Set: true, Number: 10},
	}
	missing := valid
	missing.Name = ""
	missing.Cluster = ""
	missing.UserName = "—"
	missing.Account = ""
	missing.Partition = "—"
	missing.QOS = ""
	missing.NodeList = ""
	missing.NodeCount = model.NoValNumber{}
	missing.EndTime = model.NoValNumber{}
	missing.States = []model.State{model.StatePending}
	missing.ElapsedSeconds = 0
	missing.TimeLimit = model.NoValNumber{}
	for _, column := range DefaultColumns() {
		if column.ID == "job_id" || column.ID == "state" {
			continue // JobID has no unset form; state renders an explicit UNKNOWN state.
		}
		t.Run(column.ID, func(t *testing.T) {
			if !column.Less(valid, missing) || column.Less(missing, valid) {
				t.Fatalf("unset %s value did not sort last", column.ID)
			}
		})
	}
}

func TestJobIDComparatorUsesArrayAndHetComponents(t *testing.T) {
	jobs := []model.Job{
		{ID: 100, ArrayJobID: model.NoValNumber{Set: true, Number: 100}, ArrayTaskID: model.NoValNumber{Set: true, Number: 2}},
		{ID: 100, ArrayJobID: model.NoValNumber{Set: true, Number: 100}},
		{ID: 101, HetJobID: model.NoValNumber{Set: true, Number: 100}, HetJobOffset: model.NoValNumber{Set: true, Number: 1}},
		{ID: 99},
	}
	want := []string{"99", "100", "100+1", "100_2"}
	sort.Slice(jobs, func(i, j int) bool { return lessJobID(jobs[i], jobs[j]) })
	for i := range jobs {
		if got := jobs[i].DisplayID(); got != want[i] {
			t.Fatalf("sorted ID at %d = %s, want %s", i, got, want[i])
		}
	}
}
