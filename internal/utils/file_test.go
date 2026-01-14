package utils

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDirNotExists(t *testing.T) {
	tests := []struct {
		name string
		setup func() string
		want bool
	}{
		{
			name: "directory does not exist",
			setup: func() string {
				return filepath.Join(t.TempDir(), "nonexistent")
			},
			want: true,
		},
		{
			name: "directory exists",
			setup: func() string {
				dir := filepath.Join(t.TempDir(), "exists")
				os.MkdirAll(dir, 0755)
				return dir
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			got := DirNotExists(path)
			if got != tt.want {
				t.Errorf("DirNotExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFileExists(t *testing.T) {
	tests := []struct {
		name  string
		setup func() string
		want  bool
	}{
		{
			name: "file exists",
			setup: func() string {
				tmpDir := t.TempDir()
				filePath := filepath.Join(tmpDir, "test.txt")
				os.WriteFile(filePath, []byte("test"), 0644)
				return filePath
			},
			want: true,
		},
		{
			name: "file does not exist",
			setup: func() string {
				return filepath.Join(t.TempDir(), "nonexistent.txt")
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := tt.setup()
			got := FileExists(path)
			if got != tt.want {
				t.Errorf("FileExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateDir(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		dirName string
		wantErr bool
	}{
		{
			name:    "create new directory",
			path:    filepath.Join(t.TempDir(), "newdir"),
			dirName: "test",
			wantErr: false,
		},
		{
			name:    "create nested directory",
			path:    filepath.Join(t.TempDir(), "parent", "child"),
			dirName: "nested",
			wantErr: false,
		},
		{
			name:    "directory already exists",
			path: func() string {
				dir := filepath.Join(t.TempDir(), "exists")
				os.MkdirAll(dir, 0755)
				return dir
			}(),
			dirName: "existing",
			wantErr: false, // Should not error if exists
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := CreateDir(tt.path, tt.dirName)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateDir() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !FileExists(tt.path) {
					t.Errorf("CreateDir() directory was not created: %s", tt.path)
				}
			}
		})
	}
}
