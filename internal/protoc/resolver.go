// Package protoc provides protobuf compilation and dependency resolution.
package protoc

import (
	"bytes"
	"context"
	"path"
	"strings"
	"sync"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// RegistryResolver resolves proto imports from the registry.
type RegistryResolver struct {
	cache    *registry.Cache
	snapshot git.Hash

	mu       sync.Mutex
	projects map[registry.ProjectPath]struct{} // Discovered projects
}

// NewRegistryResolver creates a new registry resolver.
func NewRegistryResolver(ctx context.Context, cache *registry.Cache, snapshot git.Hash) *RegistryResolver {
	return &RegistryResolver{
		cache:    cache,
		snapshot: snapshot,
		projects: make(map[registry.ProjectPath]struct{}),
	}
}

// FindFileByPath implements protocompile.Resolver.
func (r *RegistryResolver) FindFileByPath(filePath string) (protocompile.SearchResult, error) {
	ctx := context.Background()

	logger.Log(ctx).Debug().Str("path", filePath).Msg("Resolving import")

	// Look up project for this path
	res, err := r.cache.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     filePath,
		Snapshot: r.snapshot,
	})
	if err != nil {
		return protocompile.SearchResult{}, err
	}

	// Record discovered project
	r.mu.Lock()
	r.projects[res.Project.Path] = struct{}{}
	r.mu.Unlock()

	// Get relative path within project
	relPath := strings.TrimPrefix(filePath, string(res.Project.Path)+"/")

	// List files to find the hash
	filesRes, err := r.cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
		Project:  res.Project.Path,
		Snapshot: r.snapshot,
	})
	if err != nil {
		return protocompile.SearchResult{}, err
	}

	// Find the file
	var fileHash git.Hash
	for _, f := range filesRes.Files {
		if f.Path == relPath || f.Path == filePath {
			fileHash = f.Hash
			break
		}
	}
	if fileHash == "" {
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Read file content
	var buf bytes.Buffer
	if err := r.cache.ReadProjectFile(ctx, registry.ProjectFile{
		Snapshot: r.snapshot,
		Project:  res.Project.Path,
		Path:     relPath,
		Hash:     fileHash,
	}, &buf); err != nil {
		return protocompile.SearchResult{}, err
	}

	return protocompile.SearchResult{
		Source: bytes.NewReader(buf.Bytes()),
	}, nil
}

// DiscoveredProjects returns the list of discovered projects.
func (r *RegistryResolver) DiscoveredProjects() []registry.ProjectPath {
	r.mu.Lock()
	defer r.mu.Unlock()

	projects := make([]registry.ProjectPath, 0, len(r.projects))
	for p := range r.projects {
		projects = append(projects, p)
	}
	return projects
}

// LogReporter reports compilation errors to a logger.
type LogReporter struct {
	Log    *zerolog.Logger
	failed bool
}

// Error implements reporter.Reporter.
func (r *LogReporter) Error(err reporter.ErrorWithPos) error {
	r.failed = true
	r.Log.Error().
		Str("file", err.GetPosition().String()).
		Msg(err.Unwrap().Error())
	return nil // Continue processing
}

// Warning implements reporter.Reporter.
func (r *LogReporter) Warning(err reporter.ErrorWithPos) {
	r.Log.Warn().
		Str("file", err.GetPosition().String()).
		Msg(err.Unwrap().Error())
}

// Failed returns true if any errors were reported.
func (r *LogReporter) Failed() bool {
	return r.failed
}

// DiscoverDependencies discovers all transitive dependencies for the given proto files.
func DiscoverDependencies(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
) ([]registry.ProjectPath, error) {
	resolver := NewRegistryResolver(ctx, cache, snapshot)

	// Get all proto files from the requested projects
	var protoFiles []string
	for _, project := range projects {
		filesRes, err := cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			return nil, err
		}

		for _, f := range filesRes.Files {
			protoFiles = append(protoFiles, path.Join(string(project), f.Path))
		}

		// Mark requested projects as discovered
		resolver.mu.Lock()
		resolver.projects[project] = struct{}{}
		resolver.mu.Unlock()
	}

	if len(protoFiles) == 0 {
		return projects, nil
	}

	// Compile to discover imports
	rep := &LogReporter{Log: logger.Log(ctx)}
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(resolver),
		Reporter: rep,
	}

	// Compile files (we don't care about the result, just the side effects)
	_, err := compiler.Compile(ctx, protoFiles...)
	if err != nil && !rep.Failed() {
		// Only return error if it's not a compilation error
		logger.Log(ctx).Debug().Err(err).Msg("Compilation error during dependency discovery")
	}

	return resolver.DiscoveredProjects(), nil
}

// ValidateProtos validates that the proto files compile successfully.
func ValidateProtos(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
) error {
	log := logger.Log(ctx)
	resolver := NewRegistryResolver(ctx, cache, snapshot)

	// Get all proto files
	var protoFiles []string
	for _, project := range projects {
		filesRes, err := cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			continue
		}

		for _, f := range filesRes.Files {
			protoFiles = append(protoFiles, path.Join(string(project), f.Path))
		}
	}

	if len(protoFiles) == 0 {
		return nil
	}

	rep := &LogReporter{Log: log}
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(resolver),
		Reporter: rep,
	}

	_, err := compiler.Compile(ctx, protoFiles...)
	if rep.Failed() {
		return &CompileError{Message: ErrCompilationFailed}
	}
	if err != nil {
		return &CompileError{Message: err.Error()}
	}

	return nil
}

const (
	// ErrCompilationFailed is the error message for proto compilation failures.
	ErrCompilationFailed = "proto compilation failed"
)

// CompileError represents a compilation error.
type CompileError struct {
	Message string
}

func (e *CompileError) Error() string {
	return e.Message
}
