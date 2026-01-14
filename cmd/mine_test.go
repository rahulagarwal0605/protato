package cmd

import (
"testing"
)

func TestMineCmdFormatPath(t *testing.T) {
	tests := []struct {
		name     string
		absPath  string
		repoRoot string
		absolute bool
		want     string
	}{
		{
			name:     "relative path",
			absPath:  "/home/user/project/proto/api.proto",
			repoRoot: "/home/user/project",
			absolute: false,
			want:     "proto/api.proto",
		},
		{
			name:     "absolute path",
			absPath:  "/home/user/project/proto/api.proto",
			repoRoot: "/home/user/project",
			absolute: true,
			want:     "/home/user/project/proto/api.proto",
		},
		{
			name:     "relative path when cannot compute rel",
			absPath:  "/other/path/proto/api.proto",
			repoRoot: "/home/user/project",
			absolute: false,
			want:     "../../../other/path/proto/api.proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
cmd := &MineCmd{Absolute: tt.absolute}
			got := cmd.formatPath(tt.absPath, tt.repoRoot)
			if got != tt.want {
				t.Errorf("formatPath() = %v, want %v", got, tt.want)
			}
		})
	}
}
