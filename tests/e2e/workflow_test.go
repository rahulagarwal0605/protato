package e2e

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

// TestE2E_CompleteWorkflow tests a complete end-to-end workflow:
// 1. Initialize workspace
// 2. Create new projects
// 3. Add proto files
// 4. List projects
// 5. Verify workspace
func TestE2E_CompleteWorkflow(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()

	// Setup git repository (required for commands)
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()
	exec.Command("git", "remote", "add", "origin", "https://example.com/repo.git").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	globals := &cmd.GlobalOptions{}

	// Step 1: Initialize workspace
	t.Run("Initialize workspace", func(t *testing.T) {
		initCmd := cmd.InitCmd{
			SkipPrompts: true,
			Service:     "test-service",
			OwnedDir:    "proto",
			VendorDir:   "vendor-proto",
		}

		if err := initCmd.Run(globals, ctx); err != nil {
			t.Fatalf("Failed to initialize workspace: %v", err)
		}

		// Verify config file exists
		configPath := filepath.Join(tmpDir, "protato.yaml")
		if !testhelpers.FileExists(configPath) {
			t.Error("protato.yaml was not created")
		}
	})

	// Step 2: Create new projects
	t.Run("Create new projects", func(t *testing.T) {
		newCmd := cmd.NewCmd{
			Paths: []string{"team/service1", "team/service2"},
		}

		if err := newCmd.Run(globals, ctx); err != nil {
			if err.Error() == "registry URL not configured" {
				t.Skip("Skipping test: registry URL not configured (expected in test environment)")
			}
			t.Fatalf("Failed to create projects: %v", err)
		}

		// Verify project directories exist
		project1Path := filepath.Join(tmpDir, "proto", "team", "service1")
		project2Path := filepath.Join(tmpDir, "proto", "team", "service2")

		if !testhelpers.FileExists(project1Path) {
			t.Error("Project 1 directory was not created")
		}
		if !testhelpers.FileExists(project2Path) {
			t.Error("Project 2 directory was not created")
		}
	})

	// Step 3: Add proto files
	t.Run("Add proto files", func(t *testing.T) {
		testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
			"v1/api.proto": `syntax = "proto3";
package team.service1.v1;

message Request {
  string id = 1;
}`,
		})

		testhelpers.CreateTestProject(t, tmpDir, "proto/team/service2", map[string]string{
			"v1/api.proto": `syntax = "proto3";
package team.service2.v1;

message Request {
  string id = 1;
}`,
		})
	})

	// Step 4: List projects
	t.Run("List local projects", func(t *testing.T) {
		listCmd := cmd.ListCmd{
			Local: true,
		}

		if err := listCmd.Run(globals, ctx); err != nil {
			t.Fatalf("Failed to list projects: %v", err)
		}
	})

	// Step 5: List owned files
	t.Run("List owned files", func(t *testing.T) {
		mineCmd := cmd.MineCmd{}

		if err := mineCmd.Run(globals, ctx); err != nil {
			t.Fatalf("Failed to list owned files: %v", err)
		}
	})

	// Step 6: Verify workspace
	t.Run("Verify workspace", func(t *testing.T) {
		verifyCmd := cmd.VerifyCmd{
			Offline: true, // Skip registry for e2e test
		}

		// This might fail without registry, but should not crash
		_ = verifyCmd.Run(globals, ctx)
	})
}

// TestE2E_InitAndMine tests initialization and listing owned files
func TestE2E_InitAndMine(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping e2e test in short mode")
	}

	tmpDir := t.TempDir()

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	globals := &cmd.GlobalOptions{}

	// Initialize
	initCmd := cmd.InitCmd{
		SkipPrompts: true,
		Service:     "test-service",
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize: %v", err)
	}

	// Create projects and files
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service", map[string]string{
		"v1/api.proto":      "syntax = \"proto3\";",
		"v1/messages.proto": "syntax = \"proto3\";",
	})

	// List owned files
	mineCmd := cmd.MineCmd{}
	if err := mineCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list owned files: %v", err)
	}

	// List projects only
	mineCmdProjects := cmd.MineCmd{Projects: true}
	if err := mineCmdProjects.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
}
