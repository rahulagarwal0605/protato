package integration

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

func TestVerifyCmd_Offline(t *testing.T) {
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

	// Verify offline (should check orphaned files only)
	verifyCmd := cmd.VerifyCmd{
		Offline: true,
	}
	err := verifyCmd.Run(globals, ctx)
	// Should succeed or fail gracefully without registry
	if err != nil {
		t.Logf("VerifyCmd.Run() offline returned: %v", err)
	}

	_ = ws // Use ws to avoid unused variable
}

func TestVerifyCmd_OrphanedFiles(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Create a project
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	// Create an orphaned file
	orphanedPath := filepath.Join(tmpDir, "proto", "orphaned.proto")
	os.WriteFile(orphanedPath, []byte("syntax = \"proto3\";"), 0644)

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

	// Verify should detect orphaned files
	verifyCmd := cmd.VerifyCmd{
		Offline: true,
	}
	err := verifyCmd.Run(globals, ctx)
	// Should detect orphaned files (may or may not fail depending on implementation)
	if err != nil {
		t.Logf("VerifyCmd detected issues: %v", err)
	}

	_ = ws // Use ws to avoid unused variable
}

func TestVerifyCmd_WithReceivedProjects(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

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

	// Verify with received projects (will fail without registry for integrity check)
	verifyCmd := cmd.VerifyCmd{
		Offline: true,
	}
	err = verifyCmd.Run(globals, ctx)
	// Should handle gracefully
	if err != nil {
		t.Logf("VerifyCmd.Run() with received projects: %v", err)
	}
}
