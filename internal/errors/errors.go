// Package errors provides shared error variables used across the protato codebase.
//
// Errors are organized by domain:
//   - Workspace errors: Related to local workspace operations
//   - Registry errors: Related to registry operations
package errors

import "errors"

// Workspace errors are returned by workspace-related operations.
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

// Registry errors are returned by registry-related operations.
var (
	// ErrNotFound is returned when a project is not found.
	ErrNotFound = errors.New("project not found")
)
