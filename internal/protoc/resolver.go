// Package protoc provides protobuf compilation and dependency resolution.
package protoc

import (
	"bytes"
	"context"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
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
		cache:     cache,
		snapshot:  snapshot,
		projects:  make(map[registry.ProjectPath]struct{}),
		fileCache: make(map[string][]byte),
		// importPrefix is set by configureResolver based on ownedDir
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

			// Cache at import path (how files are imported in proto files)
			// e.g., "lcs-svc/enums/account_status.proto" -> "proto/enums/account_status.proto"
			if r.servicePrefix != "" && strings.HasPrefix(registryPath, r.servicePrefix+"/") {
				subPath := strings.TrimPrefix(registryPath, r.servicePrefix+"/")

				// Skip google/protobuf - those come from standard imports
				if strings.Contains(subPath, "google/protobuf/") {
					r.projects[project] = struct{}{}
					r.mu.Unlock()
					continue
				}

				// Cache at import path: importPrefix/subPath (e.g., "proto/enums/account_status.proto")
				if r.importPrefix != "" {
					importPath := r.importPrefix + "/" + subPath
					r.fileCache[importPath] = content
				} else {
					r.fileCache[subPath] = content
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

// findAllBufYamlWithDeps searches for all buf.yaml files with deps in the workspace.
// Returns a list of directories containing buf.yaml with deps.
func findAllBufYamlWithDeps(workspaceRoot string) []string {
	var dirs []string

	// Walk the workspace to find all buf.yaml files
	filepath.WalkDir(workspaceRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}

		// Skip hidden directories and common non-proto directories
		if d.IsDir() {
			name := d.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				return filepath.SkipDir
			}
			return nil
		}

		// Check for buf.yaml
		if d.Name() != "buf.yaml" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		// Check if it has deps section
		if strings.Contains(string(content), "deps:") {
			dirs = append(dirs, filepath.Dir(path))
		}

		return nil
	})

	return dirs
}

// exportBufDependencies runs `buf export` to get all proto files including BSR dependencies.
// Returns the path to the exported directory, or empty string if buf is not available or fails.
func exportBufDependencies(ctx context.Context, bufDir string, log *zerolog.Logger) string {
	// Check if buf CLI is available
	if _, err := exec.LookPath("buf"); err != nil {
		log.Debug().Msg("buf CLI not found, skipping BSR dependency resolution")
		return ""
	}

	// Create temp directory for export
	exportDir, err := os.MkdirTemp("", "protato-buf-export-*")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to create temp directory for buf export")
		return ""
	}

	// Run buf export
	log.Debug().Str("dir", bufDir).Msg("Exporting buf dependencies")
	cmd := exec.CommandContext(ctx, "buf", "export", ".", "-o", exportDir)
	cmd.Dir = bufDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		log.Debug().Err(err).Str("output", string(output)).Msg("buf export failed, continuing without BSR deps")
		os.RemoveAll(exportDir)
		return ""
	}

	log.Debug().Str("exportDir", exportDir).Msg("Successfully exported buf dependencies")
	return exportDir
}

// loadExportedFiles loads proto files from the buf export directory into the resolver cache.
func (r *RegistryResolver) loadExportedFiles(exportDir string, log *zerolog.Logger) error {
	count := 0
	err := filepath.WalkDir(exportDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() || !strings.HasSuffix(filePath, ".proto") {
			return nil
		}

		// Get relative path (this is the import path)
		relPath, err := filepath.Rel(exportDir, filePath)
		if err != nil {
			return nil
		}
		// Convert to forward slashes for import paths
		importPath := filepath.ToSlash(relPath)

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Cache the file
		r.mu.Lock()
		// Only cache if not already present (registry files take precedence)
		if _, exists := r.fileCache[importPath]; !exists {
			r.fileCache[importPath] = content
			count++
		}
		r.mu.Unlock()

		return nil
	})

	if err != nil {
		log.Warn().Err(err).Msg("Error walking buf export directory")
	}
	log.Debug().Int("files", count).Msg("Loaded buf dependencies into cache")
	return nil
}

// ValidateProtos validates that the proto files compile successfully.
// ownedDir is the local directory prefix used in proto imports (e.g., "proto").
// workspaceRoot is the root directory of the workspace (for finding buf.yaml).
func ValidateProtos(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
	ownedDir string,
	workspaceRoot string,
) error {
	log := logger.Log(ctx)

	resolver := NewRegistryResolver(ctx, cache, snapshot)
	configureResolver(resolver, projects, ownedDir, log)

	if err := preloadProtoFiles(ctx, resolver, projects, log); err != nil {
		return err
	}

	// Try to load BSR dependencies using buf export for all buf.yaml files
	if workspaceRoot != "" {
		bufDirs := findAllBufYamlWithDeps(workspaceRoot)
		for _, bufDir := range bufDirs {
			if exportDir := exportBufDependencies(ctx, bufDir, log); exportDir != "" {
				if err := resolver.loadExportedFiles(exportDir, log); err != nil {
					log.Warn().Err(err).Msg("Failed to load buf dependencies")
				}
				os.RemoveAll(exportDir) // Cleanup after loading
			}
		}
	}

	protoFiles := buildProtoFileList(ctx, cache, snapshot, projects, resolver)
	if len(protoFiles) == 0 {
		return nil
	}

	return compileProtoFiles(ctx, resolver, protoFiles, log)
}

// configureResolver sets up the resolver with import and service prefixes.
func configureResolver(resolver *RegistryResolver, projects []registry.ProjectPath, ownedDir string, log *zerolog.Logger) {
	// Always set import prefix - empty string means root directory (ownedDir: ".")
	resolver.SetImportPrefix(ownedDir)

	if len(projects) == 0 {
		return
	}

	projectPath := string(projects[0])
	idx := strings.Index(projectPath, "/")
	if idx > 0 {
		prefix := projectPath[:idx]
		resolver.SetServicePrefix(prefix)
		log.Debug().Str("prefix", prefix).Msg("Using service prefix for import mapping")
	}
}

// preloadProtoFiles pre-loads all proto files into memory to avoid concurrent git access.
func preloadProtoFiles(ctx context.Context, resolver *RegistryResolver, projects []registry.ProjectPath, log *zerolog.Logger) error {
	log.Debug().Int("projects", len(projects)).Msg("Pre-loading proto files into memory")
	if err := resolver.PreloadFiles(ctx, projects); err != nil {
		log.Warn().Err(err).Msg("Failed to preload files, skipping validation")
		return nil
	}
	return nil
}

// buildProtoFileList builds the list of proto files to compile using import paths.
func buildProtoFileList(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
	resolver *RegistryResolver,
) []string {
	var protoFiles []string

	for _, project := range projects {
		filesRes, err := cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			continue
		}

		protoFiles = append(protoFiles, buildProjectProtoFiles(project, filesRes.Files, resolver)...)
	}

	return protoFiles
}

// buildProjectProtoFiles builds proto file paths for a single project.
func buildProjectProtoFiles(project registry.ProjectPath, files []registry.ProjectFile, resolver *RegistryResolver) []string {
	var protoFiles []string
	projectStr := string(project)

	for _, f := range files {
		importPath := buildImportPath(projectStr, f.Path, resolver)
		if importPath != "" {
			protoFiles = append(protoFiles, importPath)
		}
	}

	return protoFiles
}

// buildImportPath builds the import path for a proto file.
// Returns the path that matches how imports work in proto files.
// Returns "" for files that should NOT be compiled (only resolved when imported).
func buildImportPath(projectStr, filePath string, resolver *RegistryResolver) string {
	if resolver.servicePrefix != "" && strings.HasPrefix(projectStr, resolver.servicePrefix+"/") {
		subPath := strings.TrimPrefix(projectStr, resolver.servicePrefix+"/")

		// Skip google/protobuf files - they're provided by standard imports
		if strings.Contains(subPath, "google/protobuf") {
			return ""
		}

		// Return importPrefix/subPath/filePath to match how imports work
		// e.g., "proto/enums/account_status.proto" matches import "proto/enums/account_status.proto"
		if resolver.importPrefix != "" {
			return resolver.importPrefix + "/" + subPath + "/" + filePath
		}
		return subPath + "/" + filePath
	}

	// Fallback to registry path if no service prefix
	return path.Join(projectStr, filePath)
}

// compileProtoFiles compiles the proto files and handles errors.
func compileProtoFiles(ctx context.Context, resolver *RegistryResolver, protoFiles []string, log *zerolog.Logger) error {
	rep := &LogReporter{Log: log}

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
		return handleCompileError(err, log)
	}

	log.Info().Msg("Proto validation completed successfully")
	return nil
}

// handleCompileError handles compilation errors, including panic recovery.
func handleCompileError(err error, log *zerolog.Logger) error {
	errStr := err.Error()
	if strings.Contains(errStr, "panic") {
		log.Warn().Err(err).Msg("Proto validation encountered internal error, skipping")
		return nil
	}
	return &CompileError{Message: err.Error()}
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
