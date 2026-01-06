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

// DirectoryConfig specifies directory paths for owned and vendor protos.
type DirectoryConfig struct {
	Owned  string `yaml:"owned,omitempty"`  // Directory for owned protos (default: "proto")
	Vendor string `yaml:"vendor,omitempty"` // Directory for consumed protos (default: "vendor-proto")
}

// Config represents the protato.yaml configuration.
type Config struct {
	Service      string          `yaml:"service,omitempty"`       // Service name for registry namespacing
	Directories  DirectoryConfig `yaml:"directories,omitempty"`   // Directory configuration
	AutoDiscover bool            `yaml:"auto_discover,omitempty"` // Auto-discover projects from owned directory
	Includes     []string        `yaml:"includes,omitempty"`      // Include patterns (glob) - when auto_discover=true: additional projects, when false: only these projects
	Excludes     []string        `yaml:"excludes,omitempty"`      // Exclude patterns (glob) - exclude projects/files matching these patterns
}

// DefaultDirectoryConfig returns the default directory configuration.
func DefaultDirectoryConfig() DirectoryConfig {
	return DirectoryConfig{
		Owned:  "proto",
		Vendor: "vendor-proto",
	}
}

// OwnedDir returns the owned directory, defaulting to "proto".
func (c *Config) OwnedDir() string {
	if c.Directories.Owned == "" {
		return "proto"
	}
	return c.Directories.Owned
}

// VendorDir returns the vendor directory, defaulting to "vendor-proto".
func (c *Config) VendorDir() string {
	if c.Directories.Vendor == "" {
		return "vendor-proto"
	}
	return c.Directories.Vendor
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
