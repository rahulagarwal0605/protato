// Package protoc provides protobuf compilation and dependency resolution.
package protoc

import (
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// ValidateProtosConfig holds configuration for ValidateProtos.
type ValidateProtosConfig struct {
	Cache         registry.CacheInterface
	Snapshot      git.Hash
	Projects      []registry.ProjectPath
	OwnedDir      string // Local directory prefix used in proto imports (e.g., "proto")
	VendorDir     string // Directory containing pulled dependencies
	WorkspaceRoot string // Root directory of the workspace (for finding buf.yaml)
	ServiceName   string // Service name from workspace configuration (e.g., "lcs-svc")
}

// CompileError represents a compilation error.
type CompileError struct {
	Message string
}

func (e *CompileError) Error() string {
	return e.Message
}
