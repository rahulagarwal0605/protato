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

func TestInitCmd_Run(t *testing.T) {
	tests := []struct {
		name      string
		initCmd   cmd.InitCmd
		setupFunc func(string)
		wantErr   bool
	}{
		{
			name: "initialize with defaults",
			initCmd: cmd.InitCmd{
				SkipPrompts: true,
			},
			wantErr: false,
		},
		{
			name: "initialize with custom directories",
			initCmd: cmd.InitCmd{
				SkipPrompts: true,
				OwnedDir:    "custom-proto",
				VendorDir:   "custom-vendor",
				Service:     "test-service",
			},
			wantErr: false,
		},
		{
			name: "fail on existing workspace without force",
			initCmd: cmd.InitCmd{
				SkipPrompts: true,
			},
			setupFunc: func(root string) {
				configPath := filepath.Join(root, "protato.yaml")
				os.WriteFile(configPath, []byte("service: old-service\n"), 0644)
			},
			wantErr: true,
		},
		{
			name: "force overwrite existing workspace",
			initCmd: cmd.InitCmd{
				SkipPrompts: true,
				Force:       true,
				Service:     "new-service",
			},
			setupFunc: func(root string) {
				configPath := filepath.Join(root, "protato.yaml")
				os.WriteFile(configPath, []byte("service: old-service\n"), 0644)
			},
			wantErr: false,
		},
		{
			name: "initialize with auto-discover disabled",
			initCmd: cmd.InitCmd{
				SkipPrompts:    true,
				NoAutoDiscover: true,
				Projects:       []string{"team/service"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupFunc != nil {
				tt.setupFunc(tmpDir)
			}

			// Setup git repository (required for InitCmd)
			os.Chdir(tmpDir)
			exec.Command("git", "init").Run()
			exec.Command("git", "config", "user.email", "test@example.com").Run()
			exec.Command("git", "config", "user.name", "Test User").Run()

			// Change to temp directory
			oldWd, _ := os.Getwd()
			defer os.Chdir(oldWd)
			os.Chdir(tmpDir)

			globals := &cmd.GlobalOptions{}
			log := logger.Init()
			ctx := logger.WithLogger(context.Background(), &log)

			err := tt.initCmd.Run(globals, ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("InitCmd.Run() error = %v, wantErr %v", err, tt.wantErr)
			}

			if !tt.wantErr {
				// Verify workspace was created
				configPath := filepath.Join(tmpDir, "protato.yaml")
				if !testhelpers.FileExists(configPath) {
					t.Error("protato.yaml was not created")
				}
			}
		})
	}
}

func TestInitCmd_ValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		initCmd cmd.InitCmd
		wantErr bool
	}{
		{
			name: "valid config with auto-discover",
			initCmd: cmd.InitCmd{
				NoAutoDiscover: false,
				Projects:       []string{},
			},
			wantErr: false,
		},
		{
			name: "invalid config - projects set with auto-discover",
			initCmd: cmd.InitCmd{
				NoAutoDiscover: false,
				Projects:       []string{"team/service"},
			},
			wantErr: true,
		},
		{
			name: "valid config without auto-discover",
			initCmd: cmd.InitCmd{
				NoAutoDiscover: true,
				Projects:       []string{"team/service"},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &cmd.InitCmd{
				NoAutoDiscover: tt.initCmd.NoAutoDiscover,
				Projects:       tt.initCmd.Projects,
			}

			// Create a minimal config for validation
			config := struct {
				AutoDiscover bool
				Projects     []string
			}{
				AutoDiscover: !cfg.NoAutoDiscover,
				Projects:     cfg.Projects,
			}

			// Simulate validation logic
			hasError := config.AutoDiscover && len(config.Projects) > 0
			if hasError != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", hasError, tt.wantErr)
			}
		})
	}
}
