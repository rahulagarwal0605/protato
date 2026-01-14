package utils

import (
	"testing"

	"github.com/rahulagarwal0605/protato/internal/constants"
)

func TestValidateProjectPath(t *testing.T) {
	tests := []struct {
		name    string
		path    string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid path",
			path:    "team/service",
			wantErr: false,
		},
		{
			name:    "valid nested path",
			path:    "team/service/v1",
			wantErr: false,
		},
		{
			name:    "empty path",
			path:    "",
			wantErr: true,
			errMsg:  constants.ErrMsgProjectPathEmpty,
		},
		{
			name:    "path with backslash",
			path:    "team\\service",
			wantErr: true,
			errMsg:  constants.ErrMsgProjectPathBackslash,
		},
		{
			name:    "path with leading slash",
			path:    "/team/service",
			wantErr: true,
			errMsg:  constants.ErrMsgProjectPathSlash,
		},
		{
			name:    "path with trailing slash",
			path:    "team/service/",
			wantErr: true,
			errMsg:  constants.ErrMsgProjectPathSlash,
		},
		{
			name:    "single segment path",
			path:    "service",
			wantErr: false,
		},
		{
			name:    "path with dots",
			path:    "team/service.v1",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateProjectPath(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateProjectPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil && tt.errMsg != "" && err.Error() != tt.errMsg {
				t.Errorf("ValidateProjectPath() error = %v, wantErrMsg %v", err.Error(), tt.errMsg)
			}
		})
	}
}

func TestProjectsOverlap(t *testing.T) {
	tests := []struct {
		name     string
		projects []string
		wantErr  bool
	}{
		{
			name:     "no overlap",
			projects: []string{"team/service1", "team/service2"},
			wantErr:  false,
		},
		{
			name:     "overlapping paths - parent and child",
			projects: []string{"team/service", "team/service/v1"},
			wantErr:  true,
		},
		{
			name:     "overlapping paths - child and parent",
			projects: []string{"team/service/v1", "team/service"},
			wantErr:  true,
		},
		{
			name:     "overlapping paths - same path",
			projects: []string{"team/service", "team/service"},
			wantErr:  true,
		},
		{
			name:     "no overlap - different prefixes",
			projects: []string{"team1/service", "team2/service"},
			wantErr:  false,
		},
		{
			name:     "single project",
			projects: []string{"team/service"},
			wantErr:  false,
		},
		{
			name:     "empty list",
			projects: []string{},
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProjectsOverlap(tt.projects)
			if (err != nil) != tt.wantErr {
				t.Errorf("ProjectsOverlap() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestPathBelongsToAny(t *testing.T) {
	tests := []struct {
		name     string
		path     string
		basePaths map[string]bool
		want     bool
	}{
		{
			name: "path belongs to base",
			path: "team/service/v1/api.proto",
			basePaths: map[string]bool{
				"team/service": true,
			},
			want: true,
		},
		{
			name: "path equals base",
			path: "team/service",
			basePaths: map[string]bool{
				"team/service": true,
			},
			want: true,
		},
		{
			name: "path does not belong",
			path: "team/service2/v1/api.proto",
			basePaths: map[string]bool{
				"team/service": true,
			},
			want: false,
		},
		{
			name: "path belongs to one of multiple bases",
			path: "team/service/v1/api.proto",
			basePaths: map[string]bool{
				"team/service":  true,
				"other/service": true,
			},
			want: true,
		},
		{
			name:     "empty base paths",
			path:     "team/service/v1/api.proto",
			basePaths: map[string]bool{},
			want:     false,
		},
		{
			name: "path prefix matches but not exact",
			path: "team/service-other/v1/api.proto",
			basePaths: map[string]bool{
				"team/service": true,
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := PathBelongsToAny(tt.path, tt.basePaths)
			if got != tt.want {
				t.Errorf("PathBelongsToAny() = %v, want %v", got, tt.want)
			}
		})
	}
}
