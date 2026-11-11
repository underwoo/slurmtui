package model

import (
	"encoding/json"
	"testing"
)

func TestNoValNumberValueAndString(t *testing.T) {
	tests := []struct {
		name      string
		json      string
		wantValue int64
		wantSet   bool
		wantText  string
	}{
		{name: "unset", json: `{"set":false,"infinite":false,"number":0}`, wantText: "—"},
		{name: "finite", json: `{"set":true,"infinite":false,"number":42}`, wantValue: 42, wantSet: true, wantText: "42"},
		{name: "infinite", json: `{"set":true,"infinite":true,"number":0}`, wantText: "∞"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var value NoValNumber
			if err := json.Unmarshal([]byte(tt.json), &value); err != nil {
				t.Fatal(err)
			}
			got, ok := value.Value()
			if got != tt.wantValue || ok != tt.wantSet {
				t.Fatalf("Value() = (%d, %t), want (%d, %t)", got, ok, tt.wantValue, tt.wantSet)
			}
			if value.String() != tt.wantText {
				t.Fatalf("String() = %q, want %q", value.String(), tt.wantText)
			}
		})
	}
}

func TestExitCodeSignalObject(t *testing.T) {
	var value ExitCode
	if err := json.Unmarshal([]byte(`{"signal":{"id":{"set":true,"infinite":false,"number":9},"name":"KILL"}}`), &value); err != nil {
		t.Fatal(err)
	}
	if got, ok := value.Signal.ID.Value(); got != 9 || !ok {
		t.Fatalf("Signal.ID.Value() = (%d, %t), want (9, true)", got, ok)
	}
	if value.Signal.Name != "KILL" {
		t.Fatalf("Signal.Name = %q, want KILL", value.Signal.Name)
	}
}
