package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRelPathToSlash(t *testing.T) {
	tmpDir := t.TempDir()
	base := filepath.Join(tmpDir, "base")
	target := filepath.Join(tmpDir, "base", "sub", "file.txt")

	if err := os.MkdirAll(filepath.Dir(target), 0755); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		base    string
		target  string
		want    string
		wantErr bool
	}{
		{
			name:    "relative path",
			base:    base,
			target:  target,
			want:    "sub/file.txt",
			wantErr: false,
		},
		{
			name:    "same path",
			base:    base,
			target:  base,
			want:    ".",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RelPathToSlash(tt.base, tt.target)
			if (err != nil) != tt.wantErr {
				t.Errorf("RelPathToSlash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("RelPathToSlash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimPathPrefix(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{
			name:   "trim prefix",
			path:   "prefix/path/to/file",
			prefix: "prefix",
			want:   "path/to/file",
		},
		{
			name:   "no prefix",
			path:   "path/to/file",
			prefix: "prefix",
			want:   "path/to/file",
		},
		{
			name:   "exact match",
			path:   "prefix",
			prefix: "prefix",
			want:   "prefix", // TrimPathPrefix only removes "prefix/" not exact match
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimPathPrefix(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("TrimPathPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractParentPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "extract parent",
			path: "a/b/c",
			want: "a/b",
		},
		{
			name: "two components",
			path: "a/b",
			want: "a",
		},
		{
			name: "single component",
			path: "a",
			want: "",
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractParentPath(tt.path)
			if got != tt.want {
				t.Errorf("ExtractParentPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRemovePathPrefixIfExists(t *testing.T) {
	tests := []struct {
		name   string
		path   string
		prefix string
		want   string
	}{
		{
			name:   "remove prefix",
			path:   "proto/common/file.proto",
			prefix: "proto",
			want:   "common/file.proto",
		},
		{
			name:   "prefix doesn't match",
			path:   "other/file.proto",
			prefix: "proto",
			want:   "",
		},
		{
			name:   "empty prefix",
			path:   "proto/file.proto",
			prefix: "",
			want:   "proto/file.proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := RemovePathPrefixIfExists(tt.path, tt.prefix)
			if got != tt.want {
				t.Errorf("RemovePathPrefixIfExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestJoinPathPrefix(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		parts  []string
		want   string
	}{
		{
			name:   "join with prefix",
			prefix: "proto",
			parts:  []string{"common", "file.proto"},
			want:   "proto/common/file.proto",
		},
		{
			name:   "single part",
			prefix: "proto",
			parts:  []string{"file.proto"},
			want:   "proto/file.proto",
		},
		{
			name:   "empty parts",
			prefix: "proto",
			parts:  []string{},
			want:   "proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinPathPrefix(tt.prefix, tt.parts...)
			if got != tt.want {
				t.Errorf("JoinPathPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAbsPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "relative path",
			path:    "test",
			wantErr: false,
		},
		{
			name:    "absolute path",
			path:    "/tmp",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AbsPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("AbsPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got == "" {
				t.Errorf("AbsPath() returned empty string")
			}
		})
	}
}
