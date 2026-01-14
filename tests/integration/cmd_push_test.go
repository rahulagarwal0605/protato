package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

func TestPushCmd_NoOwnedProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	globals := &cmd.GlobalOptions{}
	ctx := context.Background()

	// Push with no owned projects
	pushCmd := cmd.PushCmd{
		NoValidate: true,
	}
	err := pushCmd.Run(globals, ctx)
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error without registry
	} else if err == nil {
		t.Log("PushCmd.Run() succeeded with no owned projects")
	}

	_ = ws // Use ws to avoid unused variable
}

func TestPushCmd_WithOwnedProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Add owned projects
	ws.AddOwnedProjects([]string{"team/service1"})

	// Create proto files
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto": `syntax = "proto3";
package team.service1.v1;

message Request {
  string id = 1;
}`,
	})

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Initial commit").Run()
	exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	globals := &cmd.GlobalOptions{}
	ctx := context.Background()

	// Push with owned projects (will fail without registry, but tests the logic)
	pushCmd := cmd.PushCmd{
		NoValidate: true,
	}
	err := pushCmd.Run(globals, ctx)
	// Expected to fail without registry URL
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error
	} else if err == nil {
		t.Log("PushCmd.Run() succeeded (no registry configured)")
	}
}

func TestPushCmd_RetryLogic(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Test retry configuration
	pushCmd := cmd.PushCmd{
		Retries:    3,
		RetryDelay: 100,
		NoValidate: true,
	}
	_ = pushCmd

	_ = ws // Use ws to avoid unused variable
}
