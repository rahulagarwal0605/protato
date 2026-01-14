package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

func setupTestGitRepo(t *testing.T) string {
	t.Helper()
	tmpDir := t.TempDir()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)
	os.Chdir(tmpDir)

	cmd := exec.Command("git", "init")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to init git repo: %v", err)
	}

	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git config email: %v", err)
	}

	cmd = exec.Command("git", "config", "user.name", "Test User")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to set git config name: %v", err)
	}

	// Create initial commit
	os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Test"), 0644)
	
	cmd = exec.Command("git", "add", ".")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git add: %v", err)
	}

	cmd = exec.Command("git", "commit", "--no-verify", "-m", "Initial commit")
	if err := cmd.Run(); err != nil {
		t.Fatalf("Failed to git commit: %v", err)
	}

	return tmpDir
}

func TestGitRepository_Open(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if repo == nil {
		t.Fatal("Open() returned nil")
	}

	if repo.Root() != repoDir {
		t.Errorf("Root() = %v, want %v", repo.Root(), repoDir)
	}

	if repo.IsBare() {
		t.Error("IsBare() = true, want false")
	}
}

func TestGitRepository_Open_Bare(t *testing.T) {
	tmpDir := t.TempDir()
	bareDir := filepath.Join(tmpDir, "bare.git")

	os.Chdir(tmpDir)
	exec.Command("git", "init", "--bare", bareDir).Run()

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, bareDir, git.OpenOptions{Bare: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if repo == nil {
		t.Fatal("Open() returned nil")
	}

	if !repo.IsBare() {
		t.Error("IsBare() = false, want true")
	}
}

func TestGitRepository_RevHash(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	hash, err := repo.RevHash(ctx, "HEAD")
	if err != nil {
		t.Fatalf("RevHash() error = %v", err)
	}

	if hash == "" {
		t.Error("RevHash() returned empty hash")
	}
}

func TestGitRepository_RevExists(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	hash, err := repo.RevHash(ctx, "HEAD")
	if err != nil {
		t.Fatalf("RevHash() error = %v", err)
	}

	exists := repo.RevExists(ctx, string(hash))
	if !exists {
		t.Error("RevExists() = false, want true")
	}

	exists = repo.RevExists(ctx, "nonexistent123")
	if exists {
		t.Error("RevExists() = true, want false")
	}
}

func TestGitRepository_ReadTree(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	treeHash, err := repo.RevHash(ctx, "HEAD^{tree}")
	if err != nil {
		t.Fatalf("RevHash() error = %v", err)
	}

	entries, err := repo.ReadTree(ctx, git.Treeish(treeHash), git.ReadTreeOptions{})
	if err != nil {
		t.Fatalf("ReadTree() error = %v", err)
	}

	if len(entries) == 0 {
		t.Error("ReadTree() returned no entries")
	}

	// Verify README.md exists
	found := false
	for _, entry := range entries {
		if entry.Path == "README.md" {
			found = true
			break
		}
	}
	if !found {
		t.Error("ReadTree() missing README.md")
	}
}

func TestGitRepository_GetUser(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	user, err := repo.GetUser(ctx)
	if err != nil {
		t.Fatalf("GetUser() error = %v", err)
	}

	if user.Email == "" {
		t.Error("GetUser() Email is empty")
	}
	if user.Name == "" {
		t.Error("GetUser() Name is empty")
	}
}

func TestGitRepository_GetRepoURL(t *testing.T) {
	repoDir := setupTestGitRepo(t)

	// Add remote
	os.Chdir(repoDir)
	exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git").Run()

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	repo, err := git.Open(ctx, repoDir, git.OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	url, err := repo.GetRepoURL(ctx)
	if err != nil {
		t.Fatalf("GetRepoURL() error = %v", err)
	}

	if url == "" {
		t.Error("GetRepoURL() returned empty URL")
	}
}
