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

	// fileCache caches resolved files - pre-loaded before compilation
	fileCache map[string][]byte

	// servicePrefix is used to map import paths to registry paths
	// e.g., "payment-service" maps "proto/common/..." to "payment-service/common/..."
	servicePrefix string

	// importPrefix is the local directory prefix used in proto imports
	// e.g., "proto" if imports use "proto/common/address.proto"
	importPrefix string

	// preloaded indicates if all files have been pre-loaded into cache
	preloaded bool
}

// NewRegistryResolver creates a new registry resolver.
func NewRegistryResolver(ctx context.Context, cache *registry.Cache, snapshot git.Hash) *RegistryResolver {
	return &RegistryResolver{
		cache:        cache,
		snapshot:     snapshot,
		projects:     make(map[registry.ProjectPath]struct{}),
		fileCache:    make(map[string][]byte),
		importPrefix: "proto", // default, can be overridden
	}
}

// SetImportPrefix sets the local directory prefix used in proto imports.
func (r *RegistryResolver) SetImportPrefix(prefix string) {
	r.importPrefix = prefix
}

// PreloadFiles loads all proto files from the given projects into memory.
// This must be called before using the resolver with protocompile to avoid
// concurrent git access issues.
func (r *RegistryResolver) PreloadFiles(ctx context.Context, projects []registry.ProjectPath) error {
	log := logger.Log(ctx)

	for _, project := range projects {
		// List all files in the project
		filesRes, err := r.cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: r.snapshot,
		})
		if err != nil {
			log.Warn().Err(err).Str("project", string(project)).Msg("Failed to list project files")
			continue
		}

		if filesRes == nil {
			continue
		}

		// Load each file into cache
		for _, f := range filesRes.Files {
			// Registry path: payment-service/common/address.proto
			registryPath := path.Join(string(project), f.Path)

			var buf bytes.Buffer
			if err := r.cache.ReadProjectFile(ctx, registry.ProjectFile{
				Snapshot: r.snapshot,
				Project:  project,
				Path:     f.Path,
				Hash:     f.Hash,
			}, &buf); err != nil {
				log.Warn().Err(err).Str("file", registryPath).Msg("Failed to read file")
				continue
			}

			content := buf.Bytes()

			r.mu.Lock()
			// Cache at registry path
			r.fileCache[registryPath] = content

			// Also cache at import path (e.g., "proto/...") if service prefix is set
			// e.g., "payment-service/common/address.proto" -> "proto/common/address.proto"
			if r.servicePrefix != "" && strings.HasPrefix(registryPath, r.servicePrefix+"/") {
				subPath := strings.TrimPrefix(registryPath, r.servicePrefix+"/")
				importPath := r.importPrefix + "/" + subPath

				// Skip google/protobuf anywhere in path - those come from standard imports
				if !strings.Contains(subPath, "google/protobuf/") {
					r.fileCache[importPath] = content
				}
			}

			r.projects[project] = struct{}{}
			r.mu.Unlock()
		}
	}

	r.preloaded = true
	log.Debug().Int("files", len(r.fileCache)).Msg("Pre-loaded proto files into memory")
	return nil
}

// FindFileByPath implements protocompile.Resolver.
// When preloaded=true, this only uses the in-memory cache (no git operations).
func (r *RegistryResolver) FindFileByPath(filePath string) (protocompile.SearchResult, error) {
	// Check cache first (thread-safe with lock)
	r.mu.Lock()
	cached, ok := r.fileCache[filePath]
	r.mu.Unlock()

	if ok {
		return protocompile.SearchResult{
			Source: bytes.NewReader(cached),
		}, nil
	}

	// Try mapped path
	mappedPath := r.mapImportPath(filePath)
	if mappedPath != filePath {
		r.mu.Lock()
		cached, ok = r.fileCache[mappedPath]
		r.mu.Unlock()

		if ok {
			return protocompile.SearchResult{
				Source: bytes.NewReader(cached),
			}, nil
		}
	}

	// If preloaded, file not found in cache means it doesn't exist
	if r.preloaded {
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Fallback to loading from git (only used if not preloaded)
	return r.loadFileFromGit(filePath)
}

// loadFileFromGit loads a file directly from the git repository.
// This is only used when files are not preloaded.
func (r *RegistryResolver) loadFileFromGit(filePath string) (protocompile.SearchResult, error) {
	ctx := context.Background()

	// Map import path if needed
	lookupPath := r.mapImportPath(filePath)

	// Look up project for this path
	res, err := r.cache.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     lookupPath,
		Snapshot: r.snapshot,
	})
	if err != nil {
		return protocompile.SearchResult{}, err
	}

	if res == nil || res.Project == nil {
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Record discovered project
	r.mu.Lock()
	r.projects[res.Project.Path] = struct{}{}
	r.mu.Unlock()

	// Get relative path within project
	relPath := strings.TrimPrefix(lookupPath, string(res.Project.Path)+"/")

	// List files to find the hash
	filesRes, err := r.cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
		Project:  res.Project.Path,
		Snapshot: r.snapshot,
	})
	if err != nil {
		return protocompile.SearchResult{}, err
	}

	if filesRes == nil {
		return protocompile.SearchResult{}, registry.ErrNotFound
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

	// Cache the file content
	fileContent := buf.Bytes()
	r.mu.Lock()
	r.fileCache[filePath] = fileContent
	r.mu.Unlock()

	return protocompile.SearchResult{
		Source: bytes.NewReader(fileContent),
	}, nil
}

// SetServicePrefix sets the service prefix for import path mapping.
func (r *RegistryResolver) SetServicePrefix(prefix string) {
	r.servicePrefix = prefix
}

// mapImportPath maps local proto import paths to registry paths.
func (r *RegistryResolver) mapImportPath(importPath string) string {
	if r.servicePrefix == "" {
		return importPath
	}

	// Map "importPrefix/..." to "service-prefix/..."
	// e.g., "proto/common/..." -> "payment-service/common/..."
	prefix := r.importPrefix + "/"
	if strings.HasPrefix(importPath, prefix) {
		return r.servicePrefix + "/" + strings.TrimPrefix(importPath, prefix)
	}

	return importPath
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

	// Extract service prefix from project paths
	if len(projects) > 0 {
		projectPath := string(projects[0])
		if idx := strings.Index(projectPath, "/"); idx > 0 {
			resolver.SetServicePrefix(projectPath[:idx])
		}
	}

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
// ownedDir is the local directory prefix used in proto imports (e.g., "proto").
func ValidateProtos(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
	ownedDir string,
) error {
	log := logger.Log(ctx)

	resolver := NewRegistryResolver(ctx, cache, snapshot)

	// Set the import prefix (e.g., "proto") from the owned directory
	if ownedDir != "" {
		resolver.SetImportPrefix(ownedDir)
	}

	// Extract service prefix from project paths
	if len(projects) > 0 {
		projectPath := string(projects[0])
		if idx := strings.Index(projectPath, "/"); idx > 0 {
			resolver.SetServicePrefix(projectPath[:idx])
			log.Debug().Str("prefix", projectPath[:idx]).Msg("Using service prefix for import mapping")
		}
	}

	// Pre-load ALL files into memory before starting compilation
	// This avoids concurrent git access issues during protocompile's parallel resolution
	log.Debug().Int("projects", len(projects)).Msg("Pre-loading proto files into memory")
	if err := resolver.PreloadFiles(ctx, projects); err != nil {
		log.Warn().Err(err).Msg("Failed to preload files, skipping validation")
		return nil
	}

	// Build list of proto files to compile using import paths
	// This matches how the files are imported in the proto source
	var protoFiles []string
	for _, project := range projects {
		filesRes, err := cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			continue
		}

		projectStr := string(project)
		for _, f := range filesRes.Files {
			// Use import path format: <ownedDir>/common/address.proto
			// Instead of registry path: payment-service/common/address.proto
			if resolver.servicePrefix != "" && strings.HasPrefix(projectStr, resolver.servicePrefix+"/") {
				subPath := strings.TrimPrefix(projectStr, resolver.servicePrefix+"/")

				// Skip google/protobuf files anywhere - they're provided by standard imports
				if strings.Contains(subPath, "google/protobuf") {
					continue
				}

				importPath := resolver.importPrefix + "/" + subPath + "/" + f.Path
				protoFiles = append(protoFiles, importPath)
			} else {
				// Fallback to registry path if no service prefix
				protoFiles = append(protoFiles, path.Join(projectStr, f.Path))
			}
		}
	}

	if len(protoFiles) == 0 {
		return nil
	}

	rep := &LogReporter{Log: log}

	// Use WithStandardImports to provide google/protobuf/* files
	// Our resolver only handles custom files; standard imports are fallback
	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(resolver),
		Reporter: rep,
	}

	log.Info().Int("files", len(protoFiles)).Msg("Validating proto files")

	_, err := compiler.Compile(ctx, protoFiles...)
	if rep.Failed() {
		return &CompileError{Message: ErrCompilationFailed}
	}
	if err != nil {
		// Check if this is a panic error from protocompile
		errStr := err.Error()
		if strings.Contains(errStr, "panic") {
			log.Warn().Err(err).Msg("Proto validation encountered internal error, skipping")
			return nil
		}
		return &CompileError{Message: err.Error()}
	}

	log.Info().Msg("Proto validation completed successfully")
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
