package service

import (
	"testing"

	"slurmtui/internal/model"
)

func TestMergeDeduplicatesAndQueueWins(t *testing.T) {
	queue := []model.Job{{ID: 123, Name: "current", States: []model.State{model.StateRunning}}}
	recent := []model.Job{
		{ID: 123, Name: "stale", States: []model.State{model.StateCompleted}},
		{ID: 456, Name: "finished", States: []model.State{model.StateFailed}},
		{ID: 456, Name: "duplicate", States: []model.State{model.StateFailed}},
	}
	got := Merge(queue, recent)
	if len(got) != 2 {
		t.Fatalf("merged %d jobs, want 2: %+v", len(got), got)
	}
	if got[0].Name != "current" || got[0].States[0] != model.StateRunning {
		t.Fatalf("queue job did not win collision: %+v", got[0])
	}
	if got[1].ID != 456 || got[1].Name != "finished" {
		t.Fatalf("unexpected recent job after deduplication: %+v", got[1])
	}
}

func TestMergeUsesArrayAndHetDisplayIDs(t *testing.T) {
	array := model.Job{ID: 1004, ArrayJobID: model.NoValNumber{Set: true, Number: 1000}, ArrayTaskID: model.NoValNumber{Set: true, Number: 4}}
	het := model.Job{ID: 2001, HetJobID: model.NoValNumber{Set: true, Number: 2000}, HetJobOffset: model.NoValNumber{Set: true, Number: 1}}
	got := Merge([]model.Job{array}, []model.Job{{ID: 9999, ArrayJobID: array.ArrayJobID, ArrayTaskID: array.ArrayTaskID}, het})
	if len(got) != 2 {
		t.Fatalf("merged %d jobs, want two distinct displayed IDs", len(got))
	}
	if got[0].DisplayID() != "1000_4" || got[1].DisplayID() != "2000+1" {
		t.Fatalf("display IDs = %q and %q", got[0].DisplayID(), got[1].DisplayID())
	}
}
