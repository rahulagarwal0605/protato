package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

func TestListCmd_Run(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create projects
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service2", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	tests := []struct {
		name    string
		listCmd cmd.ListCmd
		wantErr bool
	}{
		{
			name: "list local projects",
			listCmd: cmd.ListCmd{
				Local: true,
			},
			wantErr: false,
		},
		{
			name: "list registry projects (offline)",
			listCmd: cmd.ListCmd{
				Local:   false,
				Offline: true,
			},
			wantErr: true, // Will fail without registry URL
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globals := &cmd.GlobalOptions{}
			ctx := context.Background()

			err := tt.listCmd.Run(globals, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	_ = ws // Use ws to avoid unused variable
}
