package ui

import (
	"strings"
	"testing"
)

func TestListPickerMovesAndSelects(t *testing.T) {
	picker := newListPicker("columns", []string{"JobID", "Name", "State"}, 1)
	if picker.Cursor() != 1 {
		t.Fatalf("initial picker cursor = %d, want 1", picker.Cursor())
	}
	picker.Move(1)
	if picker.Cursor() != 2 {
		t.Fatalf("picker cursor after move = %d, want 2", picker.Cursor())
	}
	picker.Move(10)
	if picker.Cursor() != 2 {
		t.Fatalf("picker cursor escaped list bounds: %d", picker.Cursor())
	}
	if got := picker.View(40, 12); got == "" {
		t.Fatal("picker view is empty")
	}
}

func TestCheckedListPickerShowsMarkerOnCursorRow(t *testing.T) {
	picker := newListPicker("columns", []string{"job_id", "name"}, 0)
	picker.SetChecked([]bool{true, false})
	view := picker.View(40, 12)
	if !strings.Contains(view, "> [x] job_id") || !strings.Contains(view, "  [ ] name") {
		t.Fatalf("checkbox state missing from picker view:\n%s", view)
	}
}
