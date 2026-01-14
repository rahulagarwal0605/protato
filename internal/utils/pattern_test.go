package utils

import (
	"testing"
)

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		path    string
		want    bool
	}{
		{
			name:    "exact match",
			pattern: "team/service",
			path:    "team/service",
			want:    true,
		},
		{
			name:    "no match",
			pattern: "team/service",
			path:    "other/service",
			want:    false,
		},
		{
			name:    "wildcard match",
			pattern: "team/*",
			path:    "team/service",
			want:    true,
		},
		{
			name:    "wildcard no match",
			pattern: "team/*",
			path:    "other/service",
			want:    false,
		},
		{
			name:    "double wildcard match",
			pattern: "**/service",
			path:    "team/service",
			want:    true,
		},
		{
			name:    "double wildcard nested match",
			pattern: "**/service",
			path:    "team/nested/service",
			want:    true,
		},
		{
			name:    "question mark match",
			pattern: "team/serv?ce",
			path:    "team/service",
			want:    true,
		},
		{
			name:    "question mark no match",
			pattern: "team/serv?ce",
			path:    "team/services",
			want:    false,
		},
		{
			name:    "directory pattern with trailing slash",
			pattern: "team/*",
			path:    "team/service",
			want:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MatchPattern(tt.pattern, tt.path)
			if got != tt.want {
				t.Errorf("MatchPattern(%q, %q) = %v, want %v", tt.pattern, tt.path, got, tt.want)
			}
		})
	}
}
