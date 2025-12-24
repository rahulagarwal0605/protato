// Package registry provides registry cache management functionality.
package registry

import (
	"github.com/rahulagarwal0605/protato/internal/git"
)

// ProjectPath represents a project path in the registry.
type ProjectPath string

// String returns the project path as a string.
func (p ProjectPath) String() string {
	return string(p)
}

// Project represents a project in the registry.
type Project struct {
	Path          ProjectPath // Project path (e.g., "team/service")
	Commit        git.Hash    // Source repository commit
	RepositoryURL string      // Source repository URL
}

// Config represents the protato.registry.yaml configuration.
type Config struct {
	URL       string            `yaml:"-"` // Registry URL (not in file)
	Ignores   []string          `yaml:"ignores,omitempty"`
	Committer RegistryCommitter `yaml:"committer,omitempty"`
}

// RegistryCommitter represents the committer identity for registry commits.
type RegistryCommitter struct {
	Name  string `yaml:"name"`
	Email string `yaml:"email"`
}

// ProjectMeta represents the protato.root.yaml file.
type ProjectMeta struct {
	Git ProjectMetaGit `yaml:"git"`
}

// ProjectMetaGit contains Git-specific metadata.
type ProjectMetaGit struct {
	Commit string `yaml:"commit"`
	URL    string `yaml:"url"`
}

// LookupProjectRequest contains parameters for looking up a project.
type LookupProjectRequest struct {
	Path     string   // Project path to find
	Snapshot git.Hash // Registry version (optional)
}

// LookupProjectResponse contains the result of looking up a project.
type LookupProjectResponse struct {
	Project     *Project // Found project
	Snapshot    git.Hash // Actual snapshot used
	ProjectHash git.Hash // Tree hash of the project
}

// ListProjectsOptions contains options for listing projects.
type ListProjectsOptions struct {
	Prefix   string   // Filter by path prefix
	Snapshot git.Hash // Registry snapshot
}

// ListProjectFilesRequest contains parameters for listing project files.
type ListProjectFilesRequest struct {
	Project  ProjectPath
	Snapshot git.Hash
}

// ListProjectFilesResponse contains the result of listing project files.
type ListProjectFilesResponse struct {
	Files    []ProjectFile
	Snapshot git.Hash
}

// ProjectFile represents a file in a project.
type ProjectFile struct {
	Snapshot git.Hash    // Registry snapshot
	Project  ProjectPath // Project path
	Path     string      // Relative to project
	Hash     git.Hash    // Blob hash
}

// SetProjectRequest contains parameters for updating a project.
type SetProjectRequest struct {
	Project  *Project           // Project metadata
	Files    []LocalProjectFile // Complete file list
	Snapshot git.Hash           // Base snapshot
}

// LocalProjectFile represents a local file to upload.
type LocalProjectFile struct {
	Path      string // Relative to project
	LocalPath string // Absolute filesystem path
}

// SetProjectResponse contains the result of updating a project.
type SetProjectResponse struct {
	Snapshot     git.Hash // New snapshot
	FilesChanged int
	LinesAdded   int
	LinesDeleted int
}
