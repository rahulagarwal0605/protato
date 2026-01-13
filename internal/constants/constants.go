// Package constants provides shared constants used across the protato codebase.
//
// Constants are organized by category:
//   - File names: Configuration and metadata file names
//   - Directory names: Directory paths used in the registry
//   - File extensions: File extension constants
//   - Proto-related: Constants specific to protobuf processing
//   - Error message strings: String constants for error matching/comparison
//   - Validation: Validation error message strings
package constants

// File names
const (
	// ConfigFileName is the name of the protato configuration file.
	ConfigFileName = "protato.yaml"

	// LockFileName is the name of the protato lock file.
	LockFileName = "protato.lock"

	// GitattributesName is the name of the gitattributes file.
	GitattributesName = ".gitattributes"

	// ProjectMetaFile is the name of the project metadata file in the registry.
	ProjectMetaFile = "protato.root.yaml"
)

// Directory names
const (
	// ProtosDir is the directory name for proto files in the registry.
	ProtosDir = "protos"
)

// File extensions
const (
	// ProtoFileExt is the file extension for protobuf files.
	ProtoFileExt = ".proto"
)

// Proto-related constants
const (
	// GoogleProtobufPrefix is the import path prefix for standard protobuf types.
	// These are provided by protocompile and should not be resolved from the registry.
	GoogleProtobufPrefix = "google/protobuf/"

	// ImportKeyword is the "import " keyword used in proto files.
	ImportKeyword = "import "
)

// Error message strings (for error matching/comparison)
const (
	// ErrMsgValidationFailed is the error message for validation failures.
	ErrMsgValidationFailed = "validation failed"

	// ErrMsgProjectClaim is the error message for project claim errors.
	ErrMsgProjectClaim = "project claim failed"

	// ErrMsgOwnership is the error message for ownership errors.
	ErrMsgOwnership = "project ownership failed"

	// ErrMsgCompilationFailed is the error message for proto compilation failures.
	ErrMsgCompilationFailed = "proto compilation failed"
)

// Validation error messages
const (
	// ErrMsgProjectPathEmpty is returned when a project path is empty.
	ErrMsgProjectPathEmpty = "project path cannot be empty"

	// ErrMsgProjectPathBackslash is returned when a project path contains backslashes.
	ErrMsgProjectPathBackslash = "project path cannot contain backslashes"

	// ErrMsgProjectPathSlash is returned when a project path has leading or trailing slashes.
	ErrMsgProjectPathSlash = "project path cannot have leading or trailing slashes"

	// ErrMsgProjectPathInvalid is returned when a project path is invalid.
	ErrMsgProjectPathInvalid = "invalid project path"
)
