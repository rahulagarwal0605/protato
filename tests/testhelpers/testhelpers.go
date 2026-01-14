package testhelpers

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/local"
)

// SetupTestWorkspace creates a temporary workspace for testing.
func SetupTestWorkspace(t *testing.T) (string, *local.Workspace) {
	t.Helper()
	tmpDir := t.TempDir()

	cfg := &local.Config{
		Service: "test-service",
		Directories: local.DirectoryConfig{
			Owned:  "proto",
			Vendor: "vendor-proto",
		},
		AutoDiscover: true,
		Projects:     []string{},
		Ignores:      []string{},
	}

	ctx := context.Background()
	ws, err := local.Init(ctx, tmpDir, cfg, false)
	if err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	return tmpDir, ws
}

// SetupTestWorkspaceWithConfig creates a temporary workspace with custom config.
func SetupTestWorkspaceWithConfig(t *testing.T, cfg *local.Config) (string, *local.Workspace) {
	t.Helper()
	tmpDir := t.TempDir()

	ctx := context.Background()
	ws, err := local.Init(ctx, tmpDir, cfg, false)
	if err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	return tmpDir, ws
}

// CreateTestProtoFile creates a test proto file in the given directory.
func CreateTestProtoFile(t *testing.T, dir, filename, content string) string {
	t.Helper()
	filePath := filepath.Join(dir, filename)
	if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
		t.Fatalf("Failed to create directory: %v", err)
	}
	if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to write file: %v", err)
	}
	return filePath
}

// CreateTestProject creates a test project with proto files.
func CreateTestProject(t *testing.T, baseDir, projectPath string, files map[string]string) string {
	t.Helper()
	projectDir := filepath.Join(baseDir, projectPath)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project directory: %v", err)
	}

	for filename, content := range files {
		CreateTestProtoFile(t, projectDir, filename, content)
	}

	return projectDir
}

// FileExists checks if a file exists.
func FileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ReadFile reads a file's contents.
func ReadFile(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("Failed to read file: %v", err)
	}
	return string(data)
}
