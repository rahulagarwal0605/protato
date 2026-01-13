// Package local provides workspace management functionality.
package local

import (
	"errors"
	"hash"
	"os"

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
	Projects     []string        `yaml:"projects,omitempty"`      // Project patterns (glob) - when auto_discover=false: find projects matching these patterns within owned directory
	Ignores      []string        `yaml:"ignores,omitempty"`       // Ignore patterns (glob) - ignore projects/files matching these patterns within owned directory
}

// DefaultDirectoryConfig returns the default directory configuration.
func DefaultDirectoryConfig() DirectoryConfig {
	return DirectoryConfig{
		Owned:  "proto",
		Vendor: "vendor-proto",
	}
}

var (
	// ErrOwnedDirNotSet is returned when OwnedDir is called but not configured.
	ErrOwnedDirNotSet = errors.New("owned directory not configured")
	// ErrVendorDirNotSet is returned when VendorDir is called but not configured.
	ErrVendorDirNotSet = errors.New("vendor directory not configured")
	// ErrServiceNotConfigured is returned when RegistryProjectPath is called but service is not configured.
	ErrServiceNotConfigured = errors.New("service name not configured")
	// ErrAlreadyInitialized is returned when trying to init an already initialized workspace.
	ErrAlreadyInitialized = errors.New("workspace already initialized")
	// ErrNotInitialized is returned when trying to open a non-initialized workspace.
	ErrNotInitialized = errors.New("workspace not initialized")
)

// OwnedDir returns the owned directory.
// If the configured directory is ".", returns "" (empty string) to represent root.
func (c *Config) OwnedDir() (string, error) {
	if c.Directories.Owned == "" {
		return "", ErrOwnedDirNotSet
	}
	// Treat "." as root directory (empty string)
	if c.Directories.Owned == "." {
		return "", nil
	}
	return c.Directories.Owned, nil
}

// VendorDir returns the vendor directory.
// If the configured directory is ".", returns "" (empty string) to represent root.
func (c *Config) VendorDir() (string, error) {
	if c.Directories.Vendor == "" {
		return "", ErrVendorDirNotSet
	}
	// Treat "." as root directory (empty string)
	if c.Directories.Vendor == "." {
		return "", nil
	}
	return c.Directories.Vendor, nil
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

// ProjectReceiver handles receiving files for a project.
type ProjectReceiver struct {
	ws          *Workspace
	project     ProjectPath
	projectRoot string
	snapshot    git.Hash
	changed     int
	deleted     int
}

// ProjectFileWriter handles writing a project file.
type ProjectFileWriter struct {
	file         *os.File
	hash         hash.Hash
	existingHash []byte
	onClose      func(changed bool)
}
