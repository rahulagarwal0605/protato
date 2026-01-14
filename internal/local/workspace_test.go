package local

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/errors"
)

// Helper functions to avoid import cycle with testhelpers
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func createTestProject(t *testing.T, baseDir, projectPath string, files map[string]string) {
	t.Helper()
	projectDir := filepath.Join(baseDir, projectPath)
	if err := os.MkdirAll(projectDir, 0755); err != nil {
		t.Fatalf("Failed to create project directory: %v", err)
	}
	for filename, content := range files {
		filePath := filepath.Join(projectDir, filename)
		if err := os.MkdirAll(filepath.Dir(filePath), 0755); err != nil {
			t.Fatalf("Failed to create directory: %v", err)
		}
		if err := os.WriteFile(filePath, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to write file: %v", err)
		}
	}
}

func setupTestWorkspaceWithConfig(t *testing.T, cfg *Config) (string, *Workspace) {
	t.Helper()
	tmpDir := t.TempDir()
	ctx := context.Background()
	ws, err := Init(ctx, tmpDir, cfg, false)
	if err != nil {
		t.Fatalf("Failed to initialize workspace: %v", err)
	}
	return tmpDir, ws
}

func TestWorkspace_Init(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		force     bool
		wantErr   bool
		setupFunc func(string) // Setup function to create existing files
	}{
		{
			name: "initialize new workspace",
			config: &Config{
				Service: "test-service",
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			},
			force:   false,
			wantErr: false,
		},
		{
			name: "initialize with force",
			config: &Config{
				Service: "test-service",
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			},
			force:   true,
			wantErr: false,
			setupFunc: func(root string) {
				// Create existing config
				configPath := filepath.Join(root, "protato.yaml")
				os.WriteFile(configPath, []byte("service: old-service\n"), 0644)
			},
		},
		{
			name: "fail on existing workspace without force",
			config: &Config{
				Service: "test-service",
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			},
			force:   false,
			wantErr: true,
			setupFunc: func(root string) {
				configPath := filepath.Join(root, "protato.yaml")
				os.WriteFile(configPath, []byte("service: old-service\n"), 0644)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupFunc != nil {
				tt.setupFunc(tmpDir)
			}

			ctx := context.Background()
			ws, err := Init(ctx, tmpDir, tt.config, tt.force)
			if (err != nil) != tt.wantErr {
				t.Errorf("Init() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ws == nil {
				t.Error("Init() returned nil workspace without error")
			}
			if !tt.wantErr {
				// Verify directories were created
				ownedDir, _ := ws.OwnedDir()
				if !fileExists(ownedDir) {
					t.Errorf("Owned directory not created: %s", ownedDir)
				}
				vendorDir, _ := ws.VendorDir()
				if !fileExists(vendorDir) {
					t.Errorf("Vendor directory not created: %s", vendorDir)
				}
			}
		})
	}
}

func TestWorkspace_Open(t *testing.T) {
	tests := []struct {
		name      string
		setupFunc func(string) // Setup function
		wantErr   bool
	}{
		{
			name:    "open existing workspace",
			wantErr: false,
			setupFunc: func(root string) {
				// Create workspace using Init
				cfg := &Config{
					Service: "test-service",
					Directories: DirectoryConfig{
						Owned:  "proto",
						Vendor: "vendor-proto",
					},
				}
				Init(context.Background(), root, cfg, false)
			},
		},
		{
			name:    "fail on non-existent workspace",
			wantErr: true,
			setupFunc: func(root string) {
				// Don't create workspace
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()
			if tt.setupFunc != nil {
				tt.setupFunc(tmpDir)
			}

			ctx := context.Background()
			ws, err := Open(ctx, tmpDir)
			if (err != nil) != tt.wantErr {
				t.Errorf("Open() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && ws == nil {
				t.Error("Open() returned nil workspace without error")
			}
		})
	}
}

func TestWorkspace_OwnedProjects(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		setupFunc func(string) // Setup function to create files
		want      []string
	}{
		{
			name: "discover projects with auto-discover",
			config: &Config{
				Service:      "test-service",
				AutoDiscover: true,
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			},
			setupFunc: func(root string) {
				createTestProject(t, root, "proto/team/service", map[string]string{
					"v1/api.proto": "syntax = \"proto3\";",
				})
				createTestProject(t, root, "proto/team/service2", map[string]string{
					"v1/api.proto": "syntax = \"proto3\";",
				})
			},
			want: []string{"team/service/v1", "team/service2/v1"}, // Projects are discovered at proto file locations
		},
		{
			name: "no projects found",
			config: &Config{
				Service:      "test-service",
				AutoDiscover: true,
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			},
			setupFunc: func(root string) {
				// No projects created
			},
			want: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir, _ := setupTestWorkspaceWithConfig(t, tt.config)
			if tt.setupFunc != nil {
				tt.setupFunc(tmpDir)
			}

			// Reload workspace to ensure it picks up newly created files
			ctx := context.Background()
			reloadedWs, err := Open(ctx, tmpDir)
			if err != nil {
				t.Fatalf("Failed to reload workspace: %v", err)
			}

			projects, err := reloadedWs.OwnedProjects()
			if err != nil {
				t.Fatalf("OwnedProjects() error = %v", err)
			}

			if len(projects) != len(tt.want) {
				t.Errorf("OwnedProjects() length = %v, want %v. Got: %v", len(projects), len(tt.want), projects)
			}

			projectMap := make(map[string]bool)
			for _, p := range projects {
				projectMap[string(p)] = true
			}
			for _, w := range tt.want {
				if !projectMap[w] {
					t.Errorf("OwnedProjects() missing project: %s. Found: %v", w, projects)
				}
			}
		})
	}
}

func TestWorkspace_AddOwnedProjects(t *testing.T) {
	// Use workspace with auto-discover disabled to test explicit project addition
	cfg := &Config{
		Service:      "test-service",
		AutoDiscover: false,
		Directories: DirectoryConfig{
			Owned:  "proto",
			Vendor: "vendor-proto",
		},
	}
	tmpDir, ws := setupTestWorkspaceWithConfig(t, cfg)

	tests := []struct {
		name     string
		projects []string
		wantErr  bool
	}{
		{
			name:     "add single project",
			projects: []string{"team/service"},
			wantErr:  false,
		},
		{
			name:     "add multiple projects",
			projects: []string{"team/service1", "team/service2"},
			wantErr:  false,
		},
		{
			name:     "add duplicate project",
			projects: []string{"team/service", "team/service"},
			wantErr:  false, // Should handle gracefully
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ws.AddOwnedProjects(tt.projects)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddOwnedProjects() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				// Verify project directories were created
				ownedDir, err := ws.OwnedDir()
				if err != nil {
					t.Fatalf("Failed to get owned directory: %v", err)
				}
				for _, p := range tt.projects {
					projectPath := filepath.Join(ownedDir, p)
					if !fileExists(projectPath) {
						t.Errorf("AddOwnedProjects() project directory not created: %s", projectPath)
					}
				}

				// Verify config was updated by reloading the workspace
				ctx := context.Background()
				reloadedWs, err := Open(ctx, tmpDir)
				if err != nil {
					t.Fatalf("Failed to reload workspace: %v", err)
				}

				// Verify workspace can be reloaded successfully
				_ = reloadedWs
			}
		})
	}
}

func TestWorkspace_RegistryProjectPath(t *testing.T) {
	tests := []struct {
		name         string
		service      string
		localProject ProjectPath
		want         ProjectPath
		wantErr      bool
	}{
		{
			name:         "convert to registry path",
			service:      "test-service",
			localProject: "team/service",
			want:         "test-service/team/service",
			wantErr:      false,
		},
		{
			name:         "no service configured",
			service:      "",
			localProject: "team/service",
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Service: tt.service,
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			}
			_, ws := setupTestWorkspaceWithConfig(t, cfg)

			got, err := ws.RegistryProjectPath(tt.localProject)
			if (err != nil) != tt.wantErr {
				t.Errorf("RegistryProjectPath() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("RegistryProjectPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestWorkspace_LocalProjectPath(t *testing.T) {
	tests := []struct {
		name            string
		service         string
		registryProject ProjectPath
		want            ProjectPath
	}{
		{
			name:            "strip service prefix",
			service:         "test-service",
			registryProject: "test-service/team/service",
			want:            "team/service",
		},
		{
			name:            "no service prefix",
			service:         "test-service",
			registryProject: "other-service/team/service",
			want:            "other-service/team/service", // Doesn't match, returns as-is
		},
		{
			name:            "no service configured",
			service:         "",
			registryProject: "test-service/team/service",
			want:            "test-service/team/service", // Returns as-is
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &Config{
				Service: tt.service,
				Directories: DirectoryConfig{
					Owned:  "proto",
					Vendor: "vendor-proto",
				},
			}
			_, ws := setupTestWorkspaceWithConfig(t, cfg)

			got := ws.LocalProjectPath(tt.registryProject)
			if got != tt.want {
				t.Errorf("LocalProjectPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestConfig_OwnedDir(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid owned dir",
			config: &Config{
				Directories: DirectoryConfig{
					Owned: "proto",
				},
			},
			wantErr: false,
		},
		{
			name: "empty owned dir",
			config: &Config{
				Directories: DirectoryConfig{
					Owned: "",
				},
			},
			wantErr: true,
		},
		{
			name: "root directory",
			config: &Config{
				Directories: DirectoryConfig{
					Owned: ".",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.config.OwnedDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("OwnedDir() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != errors.ErrOwnedDirNotSet {
				t.Errorf("OwnedDir() error = %v, want %v", err, errors.ErrOwnedDirNotSet)
			}
		})
	}
}

func TestConfig_VendorDir(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name: "valid vendor dir",
			config: &Config{
				Directories: DirectoryConfig{
					Vendor: "vendor-proto",
				},
			},
			wantErr: false,
		},
		{
			name: "empty vendor dir",
			config: &Config{
				Directories: DirectoryConfig{
					Vendor: "",
				},
			},
			wantErr: true,
		},
		{
			name: "root directory",
			config: &Config{
				Directories: DirectoryConfig{
					Vendor: ".",
				},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := tt.config.VendorDir()
			if (err != nil) != tt.wantErr {
				t.Errorf("VendorDir() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr && err != errors.ErrVendorDirNotSet {
				t.Errorf("VendorDir() error = %v, want %v", err, errors.ErrVendorDirNotSet)
			}
		})
	}
}

func TestDefaultDirectoryConfig(t *testing.T) {
	cfg := DefaultDirectoryConfig()
	if cfg.Owned != "proto" {
		t.Errorf("DefaultDirectoryConfig().Owned = %v, want proto", cfg.Owned)
	}
	if cfg.Vendor != "vendor-proto" {
		t.Errorf("DefaultDirectoryConfig().Vendor = %v, want vendor-proto", cfg.Vendor)
	}
}
