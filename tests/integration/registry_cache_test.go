package integration

import (
	"bytes"
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// setupTestRegistry creates a temporary Git repository for testing
func setupTestRegistry(t *testing.T) (string, string) {
	t.Helper()
	tmpDir := t.TempDir()
	registryDir := filepath.Join(tmpDir, "registry.git")

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Initialize bare git repository
	cmd := exec.Command("git", "init", "--bare", registryDir)
	cmd.Dir = tmpDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init bare repo: %v", err)
	}

	// Create initial commit with project structure
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)

	// Create protos directory structure
	protosDir := filepath.Join(workDir, "protos", "team", "service")
	os.MkdirAll(protosDir, 0755)

	// Create project metadata file
	metaFile := filepath.Join(protosDir, "protato.root.yaml")
	os.WriteFile(metaFile, []byte("service: test-service\n"), 0644)

	// Create proto file
	protoFile := filepath.Join(protosDir, "v1", "api.proto")
	os.MkdirAll(filepath.Dir(protoFile), 0755)
	os.WriteFile(protoFile, []byte("syntax = \"proto3\";\npackage team.service.v1;"), 0644)

	// Initialize git repo in work directory
	cmd = exec.Command("git", "init")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init work repo: %v", err)
	}

	// Set git config for this repo
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git config email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git config name: %v", err)
	}

	// Commit and push to bare repo
	cmd = exec.Command("git", "add", ".")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "--no-verify", "-m", "Initial commit")
	cmd.Dir = workDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("Failed to git commit: %v, output: %s", err, string(output))
	}

	cmd = exec.Command("git", "remote", "add", "origin", registryDir)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to add remote: %v", err)
	}

	// Check default branch name
	cmd = exec.Command("git", "branch", "--show-current")
	cmd.Dir = workDir
	branchOutput, _ := cmd.Output()
	branch := string(branchOutput)
	if branch == "" {
		// No branch set, use main as default
		cmd = exec.Command("git", "branch", "-M", "main")
		cmd.Dir = workDir
		cmd.Run()
		branch = "main"
	} else {
		branch = strings.TrimSpace(branch)
	}

	// Push with explicit branch creation
	cmd = exec.Command("git", "push", "origin", branch+":"+branch)
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git push: %v", err)
	}

	// Set default branch in bare repository BEFORE any clones happen
	// This ensures clones will have HEAD set correctly
	refsPath := filepath.Join(registryDir, "refs", "heads", branch)
	if _, err := os.Stat(refsPath); os.IsNotExist(err) {
		t.Fatalf("Branch ref does not exist: %s", refsPath)
	}

	// Read the commit hash from the branch ref
	refContent, err := os.ReadFile(refsPath)
	if err != nil {
		t.Fatalf("Failed to read branch ref: %v", err)
	}
	commitHash := strings.TrimSpace(string(refContent))

	// Set HEAD using symbolic-ref first (points to branch)
	cmd = exec.Command("git", "symbolic-ref", "HEAD", "refs/heads/"+branch)
	cmd.Dir = registryDir
	if err := cmd.Run(); err != nil {
		// If symbolic-ref fails, use update-ref to set HEAD directly to commit
		cmd = exec.Command("git", "update-ref", "HEAD", commitHash)
		cmd.Dir = registryDir
		if err := cmd.Run(); err != nil {
			// Final fallback: write HEAD file directly
			headFile := filepath.Join(registryDir, "HEAD")
			headContent := "ref: refs/heads/" + branch + "\n"
			if err := os.WriteFile(headFile, []byte(headContent), 0644); err != nil {
				t.Fatalf("Failed to set HEAD in bare repository: %v", err)
			}
		}
	}

	// Verify HEAD was set correctly - try both symbolic-ref and rev-parse
	cmd = exec.Command("git", "symbolic-ref", "HEAD")
	cmd.Dir = registryDir
	symRefOutput, _ := cmd.Output()
	if !strings.Contains(string(symRefOutput), "refs/heads/"+branch) {
		// If symbolic-ref doesn't work, verify rev-parse works
		cmd = exec.Command("git", "rev-parse", "HEAD")
		cmd.Dir = registryDir
		headHash, err := cmd.Output()
		if err != nil || strings.TrimSpace(string(headHash)) != commitHash {
			t.Fatalf("Failed to verify HEAD in bare repository: HEAD=%s, expected=%s, err=%v", strings.TrimSpace(string(headHash)), commitHash, err)
		}
	}

	return tmpDir, registryDir
}

func TestRegistryCache_Open(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	if cache == nil {
		t.Fatal("Open() returned nil")
	}

	if cache.URL() != registryDir {
		t.Errorf("URL() = %v, want %v", cache.URL(), registryDir)
	}
}

func TestRegistryCache_Snapshot(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	if snapshot == "" {
		t.Error("Snapshot() returned empty hash")
	}
}

func TestRegistryCache_LookupProject(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Lookup project
	req := &registry.LookupProjectRequest{
		Path:     "team/service",
		Snapshot: snapshot,
	}

	resp, err := cache.LookupProject(ctx, req)
	if err != nil {
		t.Fatalf("LookupProject() error = %v", err)
	}

	if resp == nil {
		t.Fatal("LookupProject() returned nil")
	}

	if resp.Project == nil {
		t.Fatal("LookupProject() Project is nil")
	}

	if string(resp.Project.Path) != "team/service" {
		t.Errorf("LookupProject() Project.Path = %v, want team/service", resp.Project.Path)
	}
}

func TestRegistryCache_LookupProject_NotFound(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Lookup non-existent project
	req := &registry.LookupProjectRequest{
		Path:     "nonexistent/project",
		Snapshot: snapshot,
	}

	_, err = cache.LookupProject(ctx, req)
	if err == nil {
		t.Error("LookupProject() error = nil, want error")
	}
}

func TestRegistryCache_ListProjectFiles(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// List project files
	req := &registry.ListProjectFilesRequest{
		Project:  registry.ProjectPath("team/service"),
		Snapshot: snapshot,
	}

	resp, err := cache.ListProjectFiles(ctx, req)
	if err != nil {
		t.Fatalf("ListProjectFiles() error = %v", err)
	}

	if resp == nil {
		t.Fatal("ListProjectFiles() returned nil")
	}

	if len(resp.Files) == 0 {
		t.Error("ListProjectFiles() returned no files")
	}

	// Verify proto file exists
	found := false
	for _, f := range resp.Files {
		if f.Path == "v1/api.proto" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ListProjectFiles() missing v1/api.proto")
	}
}

func TestRegistryCache_ReadProjectFile(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.Snapshot(ctx)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}

	// Get file list first
	listReq := &registry.ListProjectFilesRequest{
		Project:  registry.ProjectPath("team/service"),
		Snapshot: snapshot,
	}

	listResp, err := cache.ListProjectFiles(ctx, listReq)
	if err != nil {
		t.Fatalf("ListProjectFiles() error = %v", err)
	}

	if len(listResp.Files) == 0 {
		t.Fatal("ListProjectFiles() returned no files")
	}

	// Read first file
	file := listResp.Files[0]
	var buf bytes.Buffer
	err = cache.ReadProjectFile(ctx, file, &buf)
	if err != nil {
		t.Fatalf("ReadProjectFile() error = %v", err)
	}

	content := buf.String()
	if content == "" {
		t.Error("ReadProjectFile() returned empty content")
	}
}

func TestRegistryCache_GetSnapshot(t *testing.T) {
	tmpDir, registryDir := setupTestRegistry(t)
	cacheDir := filepath.Join(tmpDir, "cache")

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache, err := registry.Open(ctx, cacheDir, registryDir)
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}
	defer cache.Close()

	snapshot, err := cache.GetSnapshot(ctx)
	if err != nil {
		t.Fatalf("GetSnapshot() error = %v", err)
	}

	if snapshot == "" {
		t.Error("GetSnapshot() returned empty hash")
	}
}
