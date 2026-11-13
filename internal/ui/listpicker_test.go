package ui

import "testing"

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
