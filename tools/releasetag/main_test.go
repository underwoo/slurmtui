package main

import "testing"

func TestReleaseKind(t *testing.T) {
	tests := []struct {
		tag  string
		kind string
		want string
	}{
		{tag: "v0.2.0-alpha.1", kind: "prerelease", want: "prerelease"},
		{tag: "v0.2.0", kind: "stable", want: "stable"},
		{tag: "v0.2", want: ""},
		{tag: "v0.2.0-beta", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.tag, func(t *testing.T) {
			got, err := releaseKind(tt.tag)
			if tt.want == "" {
				if err == nil {
					t.Fatalf("releaseKind(%q) = %q, want error", tt.tag, got)
				}
				return
			}
			if err != nil || got != tt.want {
				t.Fatalf("releaseKind(%q) = %q, %v; want %q", tt.tag, got, err, tt.want)
			}
		})
	}
}
