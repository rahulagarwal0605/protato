package utils

import (
	"os"
	"path/filepath"
	"testing"
)

type TestConfig struct {
	Name  string `yaml:"name"`
	Value int    `yaml:"value"`
}

func TestReadYAML(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.yaml")
	content := "name: test\nvalue: 42\n"
	os.WriteFile(filePath, []byte(content), 0644)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "read valid yaml",
			path:    filePath,
			wantErr: false,
		},
		{
			name:    "read non-existent file",
			path:    filepath.Join(tmpDir, "nonexistent.yaml"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var config TestConfig
			err := ReadYAML(tt.path, &config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if config.Name != "test" || config.Value != 42 {
					t.Errorf("ReadYAML() config = %+v, want {Name: test, Value: 42}", config)
				}
			}
		})
	}
}

func TestReadYAMLFile(t *testing.T) {
	tmpDir := t.TempDir()
	filePath := filepath.Join(tmpDir, "test.yaml")
	content := "name: test\nvalue: 42\n"
	os.WriteFile(filePath, []byte(content), 0644)

	tests := []struct {
		name    string
		path    string
		wantErr bool
	}{
		{
			name:    "read valid yaml file",
			path:    filePath,
			wantErr: false,
		},
		{
			name:    "read non-existent file",
			path:    filepath.Join(tmpDir, "nonexistent.yaml"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config, err := ReadYAMLFile[TestConfig](tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadYAMLFile() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && config != nil {
				if config.Name != "test" || config.Value != 42 {
					t.Errorf("ReadYAMLFile() config = %+v, want {Name: test, Value: 42}", config)
				}
			}
		})
	}
}

func TestWriteYAML(t *testing.T) {
	tmpDir := t.TempDir()

	tests := []struct {
		name    string
		config  TestConfig
		wantErr bool
	}{
		{
			name:    "write valid yaml",
			config:  TestConfig{Name: "test", Value: 42},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filePath := filepath.Join(tmpDir, "output.yaml")
			err := WriteYAML(filePath, tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteYAML() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				if !FileExists(filePath) {
					t.Errorf("WriteYAML() file was not created: %s", filePath)
				}
				// Verify we can read it back
				readConfig, err := ReadYAMLFile[TestConfig](filePath)
				if err != nil {
					t.Errorf("WriteYAML() failed to read back: %v", err)
				}
				if readConfig.Name != tt.config.Name || readConfig.Value != tt.config.Value {
					t.Errorf("WriteYAML() read back = %+v, want %+v", readConfig, tt.config)
				}
			}
		})
	}
}
