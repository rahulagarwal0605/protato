package integration

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/tests/testhelpers"
)

func TestMineCmd_Run(t *testing.T) {
	tmpDir, ws := testhelpers.SetupTestWorkspace(t)

	// Setup git repository
	os.Chdir(tmpDir)
	exec.Command("git", "init").Run()
	exec.Command("git", "config", "user.email", "test@example.com").Run()
	exec.Command("git", "config", "user.name", "Test User").Run()

	oldWd, _ := os.Getwd()
	defer os.Chdir(oldWd)

	// Create projects with files
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service1", map[string]string{
		"v1/api.proto":      "syntax = \"proto3\";",
		"v1/messages.proto":  "syntax = \"proto3\";",
		"v2/api.proto":       "syntax = \"proto3\";",
	})
	testhelpers.CreateTestProject(t, tmpDir, "proto/team/service2", map[string]string{
		"v1/api.proto": "syntax = \"proto3\";",
	})

	tests := []struct {
		name    string
		mineCmd cmd.MineCmd
		wantErr bool
	}{
		{
			name: "list all files",
			mineCmd: cmd.MineCmd{
				Projects: false,
				Absolute: false,
			},
			wantErr: false,
		},
		{
			name: "list projects only",
			mineCmd: cmd.MineCmd{
				Projects: true,
				Absolute: false,
			},
			wantErr: false,
		},
		{
			name: "list absolute paths",
			mineCmd: cmd.MineCmd{
				Projects: false,
				Absolute: true,
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			globals := &cmd.GlobalOptions{}
			ctx := context.Background()

			err := tt.mineCmd.Run(globals, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("MineCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	_ = ws // Use ws to avoid unused variable
}
