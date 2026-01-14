package utils

import (
	"testing"
)

func TestNormalizeGitURL(t *testing.T) {
	tests := []struct {
		name string
		url  string
		want string
	}{
		{
			name: "SSH URL",
			url:  "git@github.com:org/repo.git",
			want: "https://github.com/org/repo",
		},
		{
			name: "HTTPS URL with .git",
			url:  "https://github.com/org/repo.git",
			want: "https://github.com/org/repo",
		},
		{
			name: "HTTPS URL without .git",
			url:  "https://github.com/org/repo",
			want: "https://github.com/org/repo",
		},
		{
			name: "SSH URL without .git",
			url:  "git@github.com:org/repo",
			want: "https://github.com/org/repo",
		},
		{
			name: "already normalized",
			url:  "https://github.com/org/repo",
			want: "https://github.com/org/repo",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := NormalizeGitURL(tt.url)
			if got != tt.want {
				t.Errorf("NormalizeGitURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
