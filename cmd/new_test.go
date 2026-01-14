package cmd

import (
	"testing"

	"github.com/rahulagarwal0605/protato/internal/utils"
)

// TestNewCmdValidatePaths tests the validatePaths method directly
func TestNewCmdValidatePaths(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{
			name:    "valid paths",
			paths:   []string{"team/service", "team/service2"},
			wantErr: false,
		},
		{
			name:    "empty path",
			paths:   []string{""},
			wantErr: true,
		},
		{
			name:    "invalid path - leading slash",
			paths:   []string{"/team/service"},
			wantErr: true,
		},
		{
			name:    "invalid path - trailing slash",
			paths:   []string{"team/service/"},
			wantErr: true,
		},
		{
			name:    "overlapping paths",
			paths:   []string{"team/service", "team/service/v1"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := &NewCmd{Paths: tt.paths}
			err := cmd.validatePaths()
			if (err != nil) != tt.wantErr {
				t.Errorf("validatePaths() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// TestNewCmdValidatePathsLogic tests the validation logic (same as before, for backwards compatibility)
func TestNewCmdValidatePathsLogic(t *testing.T) {
	tests := []struct {
		name    string
		paths   []string
		wantErr bool
	}{
		{
			name:    "valid paths",
			paths:   []string{"team/service", "team/service2"},
			wantErr: false,
		},
		{
			name:    "empty path",
			paths:   []string{""},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Test validation logic directly (same as NewCmd.validatePaths)
			var hasErr bool
			for _, p := range tt.paths {
				if err := utils.ValidateProjectPath(p); err != nil {
					hasErr = true
					break
				}
			}
			if !hasErr {
				if err := utils.ProjectsOverlap(tt.paths); err != nil {
					hasErr = true
				}
			}

			if hasErr != tt.wantErr {
				t.Errorf("validatePaths() error = %v, wantErr %v", hasErr, tt.wantErr)
			}
		})
	}
}
