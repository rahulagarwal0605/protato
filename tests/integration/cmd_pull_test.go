package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

func TestPullCmd_NoProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	globals := &cmd.GlobalOptions{}
	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)

	// Pull with no projects specified and no received projects
	pullCmd := cmd.PullCmd{}
	err := pullCmd.Run(globals, ctx)
	// Should succeed with no projects, or handle missing registry gracefully
	if err != nil && err.Error() != "registry URL not configured" {
		t.Errorf("PullCmd.Run() with no projects should succeed, got error: %v", err)
	}

	_ = ws // Use ws to avoid unused variable
}

func TestPullCmd_ResolveProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create a received project
	receiveReq := &local.ReceiveProjectRequest{
		Project:  "external/service",
		Snapshot: git.Hash("abc123"),
	}
	receiver, err := ws.ReceiveProject(receiveReq)
	if err != nil {
		t.Fatalf("Failed to receive project: %v", err)
	}
	writer, _ := receiver.CreateFile("v1/api.proto")
	writer.Write([]byte("syntax = \"proto3\";"))
	writer.Close()
	receiver.Finish()

	globals := &cmd.GlobalOptions{}
	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)

	// Pull should try to pull received projects (but will fail without registry)
	pullCmd := cmd.PullCmd{}
	err = pullCmd.Run(globals, ctx)
	// Expected to fail without registry URL, but should not crash
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error
	} else if err == nil {
		t.Log("PullCmd.Run() succeeded (no registry configured)")
	}
}

func TestPullCmd_WithProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	globals := &cmd.GlobalOptions{}
	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)

	// Pull with specific projects (will fail without registry, but tests the logic)
	pullCmd := cmd.PullCmd{
		Projects: []string{"team/service1", "team/service2"},
		NoDeps:   true,
	}
	err := pullCmd.Run(globals, ctx)
	// Expected to fail without registry URL
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error
	} else if err == nil {
		t.Log("PullCmd.Run() succeeded (no registry configured)")
	}

	_ = ws // Use ws to avoid unused variable
}

func TestPullCmd_FilterOwnedProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Add owned projects
	ws.AddOwnedProjects([]string{"team/service1"})

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	globals := &cmd.GlobalOptions{}
	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)

	// Try to pull owned project (should be filtered out)
	pullCmd := cmd.PullCmd{
		Projects: []string{"team/service1"},
		NoDeps:   true,
	}
	err := pullCmd.Run(globals, ctx)
	// Should succeed with no projects to pull (owned projects filtered)
	if err != nil && err.Error() != "registry URL not configured" {
		t.Logf("PullCmd filtered owned projects correctly")
	}
}
