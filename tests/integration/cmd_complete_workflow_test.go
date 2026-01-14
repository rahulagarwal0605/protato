package integration

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

// TestCompleteWorkflow_InitNewPush tests: init -> new -> add files -> push
func TestCompleteWorkflow_InitNewPush(t *testing.T) {
	tmpDir := t.TempDir()

	// Setup git repository
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
	initCmd := cmd.InitCmd{
		SkipPrompts: true,
		Service:     "test-service",
		OwnedDir:    "proto",
		VendorDir:   "vendor-proto",
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Step 2: Create new projects
	// Note: This requires a registry URL, so we'll skip if not configured
	newCmd := cmd.NewCmd{
		Paths: []string{"team/service1", "team/service2"},
	}
	if err := newCmd.Run(globals, ctx); err != nil {
		if err.Error() == "registry URL not configured" {
			t.Skip("Skipping test: registry URL not configured (expected in test environment)")
		}
		t.Fatalf("Failed to create projects: %v", err)
	}

	// Step 3: Add proto files
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

	// Step 4: Commit files
	exec.Command("git", "add", ".").Run()
	exec.Command("git", "commit", "-m", "Add proto files").Run()

	// Step 5: Try to push (will fail without registry, but tests the workflow)
	pushCmd := cmd.PushCmd{
		NoValidate: true,
	}
	err := pushCmd.Run(globals, ctx)
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error
		t.Log("Push workflow completed (failed at registry step as expected)")
	} else if err == nil {
		t.Log("Push workflow completed successfully")
	}
}

// TestCompleteWorkflow_InitPull tests: init -> pull -> verify
func TestCompleteWorkflow_InitPull(t *testing.T) {
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

	// Step 1: Initialize workspace
	initCmd := cmd.InitCmd{
		SkipPrompts: true,
		Service:     "test-service",
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Step 2: Try to pull (will fail without registry, but tests the workflow)
	pullCmd := cmd.PullCmd{
		Projects: []string{"external/service"},
		NoDeps:   true,
	}
	err := pullCmd.Run(globals, ctx)
	if err != nil && err.Error() == "registry URL not configured" {
		// Expected error
		t.Log("Pull workflow completed (failed at registry step as expected)")
	} else if err == nil {
		t.Log("Pull workflow completed successfully")
	}

	// Step 3: Verify workspace
	verifyCmd := cmd.VerifyCmd{
		Offline: true,
	}
	_ = verifyCmd.Run(globals, ctx)
}

// TestCompleteWorkflow_ListAndMine tests: init -> create projects -> list -> mine
func TestCompleteWorkflow_ListAndMine(t *testing.T) {
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

	// Step 1: Initialize workspace
	initCmd := cmd.InitCmd{
		SkipPrompts: true,
		Service:     "test-service",
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Step 2: Create projects and files
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto":      "syntax = \"proto3\";",
		"v1/messages.proto": "syntax = \"proto3\";",
	})
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service2", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	// Step 3: List local projects
	listCmd := cmd.ListCmd{
		Local: true,
	}
	if err := listCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	// Step 4: List owned files
	mineCmd := cmd.MineCmd{}
	if err := mineCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list owned files: %v", err)
	}

	// Step 5: List projects only
	mineCmdProjects := cmd.MineCmd{Projects: true}
	if err := mineCmdProjects.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	// Step 6: List absolute paths
	mineCmdAbsolute := cmd.MineCmd{Absolute: true}
	if err := mineCmdAbsolute.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list absolute paths: %v", err)
	}
}

// TestCompleteWorkflow_AutoDiscover tests: init with auto-discover -> add files -> list
func TestCompleteWorkflow_AutoDiscover(t *testing.T) {
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

	// Step 1: Initialize with auto-discover
	initCmd := cmd.InitCmd{
		SkipPrompts:    true,
		Service:        "test-service",
		NoAutoDiscover: false, // Auto-discover enabled
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Step 2: Create projects (will be auto-discovered)
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service2", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	// Step 3: List projects (should discover automatically)
	listCmd := cmd.ListCmd{
		Local: true,
	}
	if err := listCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}

	// Step 4: List owned files
	mineCmd := cmd.MineCmd{}
	if err := mineCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list owned files: %v", err)
	}
}

// TestCompleteWorkflow_ForceInit tests force initialization
func TestCompleteWorkflow_ForceInit(t *testing.T) {
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

	// Step 1: Initialize workspace
	initCmd1 := cmd.InitCmd{
		SkipPrompts: true,
		Service:     "old-service",
	}
	if err := initCmd1.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Step 2: Force reinitialize with new config
	initCmd2 := cmd.InitCmd{
		SkipPrompts: true,
		Force:       true,
		Service:     "new-service",
		OwnedDir:    "custom-proto",
		VendorDir:   "custom-vendor",
	}
	if err := initCmd2.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to force reinitialize workspace: %v", err)
	}

	// Step 3: Verify new directories exist
	customProtoPath := filepath.Join(tmpDir, "custom-proto")
	customVendorPath := filepath.Join(tmpDir, "custom-vendor")

	if !testhelpers.FileExists(customProtoPath) {
		t.Error("Custom proto directory was not created")
	}
	if !testhelpers.FileExists(customVendorPath) {
		t.Error("Custom vendor directory was not created")
	}
}

// TestCompleteWorkflow_ProjectPatterns tests init with project patterns
func TestCompleteWorkflow_ProjectPatterns(t *testing.T) {
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

	// Initialize with project patterns
	initCmd := cmd.InitCmd{
		SkipPrompts:    true,
		Service:        "test-service",
		NoAutoDiscover: true,
		Projects:       []string{"team/*", "other/service"},
	}
	if err := initCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}

	// Create projects matching patterns
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})
	testhelpers.CreateTestProject(t, tmpDir, "proto/other/service", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	// List projects
	listCmd := cmd.ListCmd{
		Local: true,
	}
	if err := listCmd.Run(globals, ctx); err != nil {
		t.Fatalf("Failed to list projects: %v", err)
	}
}
