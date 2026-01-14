package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

func TestProtocResolver_WithRealCache(t *testing.T) {
	// Save and restore working directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Setup test registry
	tmpDir := t.TempDir()
	registryDir := filepath.Join(tmpDir, "registry.git")

	// Initialize bare git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init", "--bare", registryDir).Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create initial commit with project structure
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)
	os.Chdir(workDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create protos directory structure
	// Use test-service as the project prefix to match the service prefix
	protosDir := filepath.Join(workDir, "protos", "test-service", "api")
	os.MkdirAll(protosDir, 0755)

	// Create project metadata file
	metaFile := filepath.Join(protosDir, "protato.root.yaml")
	os.WriteFile(metaFile, []byte("service: test-service\n"), 0644)

	// Create proto file
	protoFile := filepath.Join(protosDir, "v1", "api.proto")
	os.MkdirAll(filepath.Dir(protoFile), 0755)
	os.WriteFile(protoFile, []byte("syntax = \"proto3\";\npackage test_service.api.v1;"), 0644)

	// Commit and push to bare repo
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "--no-verify", "-m", "Initial commit")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
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

	// Create resolver
	resolver := protoc.NewRegistryResolver(ctx, cache, snapshot)
	resolver.SetServicePrefix("test-service")
	resolver.SetImportPrefix("proto")

	// Preload files
	projects := []registry.ProjectPath{"test-service/api"}
	err = resolver.PreloadFiles(ctx, projects, false)
	if err != nil {
		t.Fatalf("PreloadFiles() error = %v", err)
	}

	// Find file
	result, err := resolver.FindFileByPath("proto/api/v1/api.proto")
	if err != nil {
		t.Fatalf("FindFileByPath() error = %v", err)
	}

	if result.Source == nil {
		t.Fatal("FindFileByPath() Source is nil")
	}

	// Read content
	buf := make([]byte, 1024)
	n, err := result.Source.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Read() error = %v", err)
	}

	if n == 0 {
		t.Error("Read() returned no content")
	}

	// Verify discovered projects
	discovered := resolver.DiscoveredProjects()
	if len(discovered) == 0 {
		t.Error("DiscoveredProjects() returned no projects")
	}
}

func TestProtocResolver_DiscoveredProjects(t *testing.T) {
	// Save and restore working directory
	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	tmpDir := t.TempDir()
	registryDir := filepath.Join(tmpDir, "registry.git")

	// Initialize bare git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init", "--bare", registryDir).Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	// Create initial commit with project structure
	workDir := filepath.Join(tmpDir, "work")
	os.MkdirAll(workDir, 0755)
	os.Chdir(workDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

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

	// Commit and push to bare repo
	cmd := exec.Command("git", "add", ".")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "--no-verify", "-m", "Initial commit")
	cmd.Dir = workDir
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
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

	resolver := protoc.NewRegistryResolver(ctx, cache, snapshot)

	// Preload files should discover projects
	projects := []registry.ProjectPath{"team/service"}
	err = resolver.PreloadFiles(ctx, projects, false)
	if err != nil {
		t.Fatalf("PreloadFiles() error = %v", err)
	}

	discovered := resolver.DiscoveredProjects()
	if len(discovered) == 0 {
		t.Error("DiscoveredProjects() returned no projects")
	}

	found := false
	for _, p := range discovered {
		if p == "team/service" {
			found = true
			break
		}
	}
	if !found {
		t.Error("DiscoveredProjects() missing team/service")
	}
}
