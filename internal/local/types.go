// Package local provides workspace management functionality.
package local

import (
	"github.com/rahulagarwal0605/protato/internal/git"
)

// ProjectPath represents a project path in the registry.
type ProjectPath string

// String returns the project path as a string.
func (p ProjectPath) String() string {
	return string(p)
}

// Config represents the protato.yaml configuration.
type Config struct {
	Projects []string `yaml:"projects,omitempty"`
	Ignores  []string `yaml:"ignores,omitempty"`
}

// LockFile represents the protato.lock file.
type LockFile struct {
	Snapshot string `yaml:"snapshot"`
}

// ProjectFile represents a file in a project.
type ProjectFile struct {
	Path         string // Relative to project root
	AbsolutePath string // Full filesystem path
}

// ReceivedProject represents a project that was pulled from the registry.
type ReceivedProject struct {
	Project          ProjectPath
	ProviderSnapshot string // Registry Git commit hash
}

// InitOptions contains options for initializing a workspace.
type InitOptions struct {
	Force    bool     // Force overwrite existing config
	Projects []string // Initial projects to claim
}

// OpenOptions contains options for opening a workspace.
type OpenOptions struct {
	// CreateIfMissing creates the config if it doesn't exist
	CreateIfMissing bool
}

// ReceiveProjectRequest contains parameters for receiving a project.
type ReceiveProjectRequest struct {
	Project  ProjectPath // Project to receive
	Snapshot git.Hash    // Registry snapshot
}

// ReceiveStats contains statistics about a receive operation.
type ReceiveStats struct {
	FilesChanged int
	FilesDeleted int
}
