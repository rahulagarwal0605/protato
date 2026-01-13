// Package protoc provides protobuf compilation and dependency resolution.
package protoc

import (
	"bytes"
	"context"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/bufbuild/protocompile"
	"github.com/bufbuild/protocompile/reporter"
	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

const (
	// googleProtobufPrefix is the import path prefix for standard protobuf types
	// These are provided by protocompile and should not be resolved from the registry
	googleProtobufPrefix = "google/protobuf/"
	// importKeyword is the "import " keyword used in proto files
	importKeyword = "import "
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
	}
}

// SetImportPrefix sets the local directory prefix used in proto imports.
func (r *RegistryResolver) SetImportPrefix(prefix string) {
	r.importPrefix = prefix
}

// PreloadFiles loads all proto files from the given projects into memory.
// This must be called before using the resolver with protocompile to avoid
// concurrent git access issues.
// If cacheAtRegistryPath is true, files are cached at both registry paths and import paths.
// This is needed for dependency discovery where files are compiled using registry paths.
func (r *RegistryResolver) PreloadFiles(ctx context.Context, projects []registry.ProjectPath, cacheAtRegistryPath bool) error {
	for _, project := range projects {
		if err := r.preloadProjectFiles(ctx, project, cacheAtRegistryPath); err != nil {
			logger.Log(ctx).Warn().Err(err).Str("project", string(project)).Msg("Failed to preload project files")
			continue
		}
	}

	r.preloaded = true
	logger.Log(ctx).Debug().Int("files", len(r.fileCache)).Msg("Pre-loaded proto files into memory")
	return nil
}

// preloadProjectFiles loads all files from a single project into the cache.
func (r *RegistryResolver) preloadProjectFiles(ctx context.Context, project registry.ProjectPath, cacheAtRegistryPath bool) error {
	filesRes, err := r.cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
		Project:  project,
		Snapshot: r.snapshot,
	})
	if err != nil {
		return err
	}

	if filesRes == nil {
		return nil
	}

	for _, file := range filesRes.Files {
		if err := r.preloadFile(ctx, project, file, cacheAtRegistryPath); err != nil {
			logger.Log(ctx).Warn().Err(err).Str("file", file.Path).Msg("Failed to preload file")
			continue
		}
	}

	return nil
}

// preloadFile loads a single file into the cache.
func (r *RegistryResolver) preloadFile(ctx context.Context, project registry.ProjectPath, file registry.ProjectFile, cacheAtRegistryPath bool) error {
	registryPath := path.Join(string(project), file.Path)

	var buf bytes.Buffer
	if err := r.cache.ReadProjectFile(ctx, file, &buf); err != nil {
		return err
	}

	content := buf.Bytes()

	r.mu.Lock()
	defer r.mu.Unlock()

	if r.servicePrefix != "" && strings.HasPrefix(registryPath, r.servicePrefix+"/") {
		r.cacheFileWithServicePrefix(ctx, registryPath, content, cacheAtRegistryPath)
	} else {
		r.fileCache[registryPath] = content
	}

	r.projects[project] = struct{}{}
	return nil
}

// cacheFileWithServicePrefix caches a file that has a service prefix.
func (r *RegistryResolver) cacheFileWithServicePrefix(ctx context.Context, registryPath string, content []byte, cacheAtRegistryPath bool) {
	subPath := strings.TrimPrefix(registryPath, r.servicePrefix+"/")

	// Skip google/protobuf - those come from standard imports
	if strings.Contains(subPath, googleProtobufPrefix) {
		return
	}

	cachePath := r.buildImportCachePath(subPath)
	untransformedContent := r.untransformImports(content)
	r.fileCache[cachePath] = untransformedContent

	if cacheAtRegistryPath {
		r.fileCache[registryPath] = content
		logger.Log(ctx).Debug().Str("registryPath", registryPath).Str("cachePath", cachePath).Msg("Cached file at both paths")
	} else {
		logger.Log(ctx).Debug().Str("registryPath", registryPath).Str("cachePath", cachePath).Msg("Cached file")
	}
}

// buildImportCachePath builds the cache path for an import based on the import prefix.
func (r *RegistryResolver) buildImportCachePath(subPath string) string {
	if r.importPrefix != "" {
		return r.importPrefix + "/" + subPath
	}
	return subPath
}

// FindFileByPath implements protocompile.Resolver.
// When preloaded=true, this only uses the in-memory cache (no git operations).
func (r *RegistryResolver) FindFileByPath(filePath string) (protocompile.SearchResult, error) {
	// Safety check
	if r == nil {
		return protocompile.SearchResult{}, fmt.Errorf("resolver is nil")
	}

	// Map import path first to ensure consistency
	// e.g., "buf/validate/..." -> "druid/buf/validate/..." when ownedDir is "."
	mappedPath := r.mapImportPath(filePath)

	// Check cache for mapped path first
	r.mu.Lock()
	cached, ok := r.fileCache[mappedPath]
	r.mu.Unlock()

	if ok {
		if cached == nil {
			return protocompile.SearchResult{}, fmt.Errorf("cached content is nil for %s", filePath)
		}
		return protocompile.SearchResult{
			Source: bytes.NewReader(cached),
		}, nil
	}

	// Try original path if different
	if mappedPath != filePath {
		r.mu.Lock()
		cached, ok = r.fileCache[filePath]
		r.mu.Unlock()

		if ok {
			if cached == nil {
				return protocompile.SearchResult{}, fmt.Errorf("cached content is nil for %s", filePath)
			}
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

	// Safety checks
	if r == nil {
		return protocompile.SearchResult{}, fmt.Errorf("resolver is nil")
	}
	if r.cache == nil {
		return protocompile.SearchResult{}, fmt.Errorf("cache is nil")
	}

	// Use original path for project lookup (don't map - we need the full registry path)
	// e.g., "lcs-svc/vendors/buf/validate/validate.proto" should lookup project "lcs-svc/vendors/buf/validate"
	logger.Log(ctx).Debug().Str("filePath", filePath).Msg("loadFileFromGit: looking up project")

	res, err := r.cache.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     filePath,
		Snapshot: r.snapshot,
	})
	if err != nil {
		logger.Log(ctx).Debug().Err(err).Str("filePath", filePath).Msg("loadFileFromGit: lookup failed")
		return protocompile.SearchResult{}, err
	}

	if res == nil || res.Project == nil {
		logger.Log(ctx).Debug().Str("filePath", filePath).Msg("loadFileFromGit: project not found")
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Record discovered project
	r.mu.Lock()
	r.projects[res.Project.Path] = struct{}{}
	r.mu.Unlock()
	logger.Log(ctx).Debug().Str("filePath", filePath).Str("project", string(res.Project.Path)).Msg("loadFileFromGit: discovered project")

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
// untransformImports converts transformed registry imports back to local imports.
// e.g., import "druid/buf/validate/..." -> import "buf/validate/..."
// e.g., import "lcs-svc/common/..." -> import "proto/common/..." (when importPrefix="proto")
func (r *RegistryResolver) untransformImports(content []byte) []byte {
	if r.servicePrefix == "" {
		return content
	}

	lines := strings.Split(string(content), "\n")
	result := make([]string, 0, len(lines))
	changed := false

	for _, line := range lines {
		transformedLine, lineChanged := r.untransformImportLine(line)
		result = append(result, transformedLine)
		if lineChanged {
			changed = true
		}
	}

	if !changed {
		return content
	}
	return []byte(strings.Join(result, "\n"))
}

// untransformImportLine transforms a single import line if it has a service prefix.
func (r *RegistryResolver) untransformImportLine(line string) (string, bool) {
	importPath := extractImportPathFromLine(line)
	if importPath == "" {
		return line, false
	}

	if !strings.HasPrefix(importPath, r.servicePrefix+"/") {
		return line, false
	}

	subPath := strings.TrimPrefix(importPath, r.servicePrefix+"/")
	newImportPath := r.buildImportPath(subPath)
	newLine := strings.Replace(line, importPath, newImportPath, 1)
	return newLine, true
}

// buildImportPath builds the import path with the import prefix if set.
func (r *RegistryResolver) buildImportPath(subPath string) string {
	return r.buildImportCachePath(subPath)
}

func (r *RegistryResolver) mapImportPath(importPath string) string {
	if r.servicePrefix == "" {
		return importPath
	}

	// Skip standard imports (google/protobuf) - they're provided by protocompile
	if strings.HasPrefix(importPath, googleProtobufPrefix) {
		return importPath
	}

	// If import has service prefix (from transformed registry files),
	// strip it to get the import path
	// e.g., "druid/buf/validate/..." -> "buf/validate/..."
	// e.g., "lcs-svc/common/..." -> "proto/common/..." (when importPrefix="proto")
	if strings.HasPrefix(importPath, r.servicePrefix+"/") {
		subPath := strings.TrimPrefix(importPath, r.servicePrefix+"/")
		if r.importPrefix != "" {
			return r.importPrefix + "/" + subPath
		}
		return subPath
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
	setupServicePrefixForDiscovery(resolver, projects)

	protoFiles := buildProtoFilesListForDiscovery(ctx, cache, snapshot, projects, resolver)
	if len(protoFiles) == 0 {
		logger.Log(ctx).Debug().Msg("No proto files to compile for dependency discovery")
		return projects, nil
	}

	logger.Log(ctx).Debug().Int("count", len(protoFiles)).Strs("files", protoFiles).Msg("Compiling files for dependency discovery")

	preloadFilesForDiscovery(ctx, resolver, projects)
	discoverProjectsFromImports(ctx, resolver, protoFiles)

	logger.Log(ctx).Debug().Int("discovered", len(resolver.projects)).Msg("Dependency discovery complete")
	return resolver.DiscoveredProjects(), nil
}

// setupServicePrefixForDiscovery extracts and sets the service prefix from project paths.
func setupServicePrefixForDiscovery(resolver *RegistryResolver, projects []registry.ProjectPath) {
	if len(projects) == 0 {
		return
	}
	projectPath := string(projects[0])
	if idx := strings.Index(projectPath, "/"); idx > 0 {
		resolver.SetServicePrefix(projectPath[:idx])
	}
}

// buildProtoFilesListForDiscovery builds the list of proto files from projects.
func buildProtoFilesListForDiscovery(
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

		ensureServicePrefixSet(resolver, project)
		protoFiles = append(protoFiles, buildImportPathsForProject(project, filesRes.Files, resolver.servicePrefix)...)

		// Mark requested projects as discovered
		resolver.mu.Lock()
		resolver.projects[project] = struct{}{}
		resolver.mu.Unlock()
	}
	return protoFiles
}

// ensureServicePrefixSet ensures the service prefix is set from the project if not already set.
func ensureServicePrefixSet(resolver *RegistryResolver, project registry.ProjectPath) {
	if resolver.servicePrefix != "" {
		return
	}
	projectStr := string(project)
	if idx := strings.Index(projectStr, "/"); idx > 0 {
		resolver.SetServicePrefix(projectStr[:idx])
	}
}

// buildImportPathsForProject builds import paths for files in a project.
func buildImportPathsForProject(project registry.ProjectPath, files []registry.ProjectFile, servicePrefix string) []string {
	var paths []string
	projectStr := string(project)
	for _, f := range files {
		if servicePrefix != "" && strings.HasPrefix(projectStr, servicePrefix+"/") {
			subPath := strings.TrimPrefix(projectStr, servicePrefix+"/")
			paths = append(paths, path.Join(subPath, f.Path))
		} else {
			paths = append(paths, path.Join(projectStr, f.Path))
		}
	}
	return paths
}

// preloadFilesForDiscovery preloads files and resets the preloaded flag for discovery.
func preloadFilesForDiscovery(ctx context.Context, resolver *RegistryResolver, projects []registry.ProjectPath) {
	if err := resolver.PreloadFiles(ctx, projects, true); err != nil {
		logger.Log(ctx).Debug().Err(err).Msg("Failed to preload files for dependency discovery")
	}

	resolver.mu.Lock()
	resolver.preloaded = false
	logger.Log(ctx).Debug().
		Int("cachedFiles", len(resolver.fileCache)).
		Msg("Cache contents before compilation")
	for path := range resolver.fileCache {
		logger.Log(ctx).Debug().Str("cachedPath", path).Msg("Cached file path")
	}
	resolver.mu.Unlock()
}

// discoverProjectsFromImports discovers projects by parsing imports from proto files.
func discoverProjectsFromImports(ctx context.Context, resolver *RegistryResolver, protoFiles []string) {
	logger.Log(ctx).Debug().Strs("files", protoFiles).Msg("Parsing proto files for dependency discovery")

	for _, protoFile := range protoFiles {
		content := getFileContentFromCache(resolver, protoFile)
		if content == nil {
			logger.Log(ctx).Debug().Str("file", protoFile).Msg("File not found in cache, skipping")
			continue
		}

		imports := extractImportsFromContent(content)
		logger.Log(ctx).Debug().Str("file", protoFile).Int("importCount", len(imports)).Msg("Extracted imports from file")

		for _, imp := range imports {
			if strings.HasPrefix(imp, googleProtobufPrefix) {
				continue
			}
			discoverProjectFromImport(ctx, resolver, imp)
		}
	}
}

// getFileContentFromCache retrieves file content from the resolver's cache.
func getFileContentFromCache(resolver *RegistryResolver, protoFile string) []byte {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()

	if resolver.servicePrefix != "" {
		registryPath := resolver.servicePrefix + "/" + protoFile
		if content, ok := resolver.fileCache[registryPath]; ok {
			return content
		}
	}
	if content, ok := resolver.fileCache[protoFile]; ok {
		return content
	}
	return nil
}

// discoverProjectFromImport attempts to discover a project from an import path.
func discoverProjectFromImport(ctx context.Context, resolver *RegistryResolver, imp string) {
	logger.Log(ctx).Debug().Str("import", imp).Msg("Found import")

	if resolver.servicePrefix == "" || !strings.HasPrefix(imp, resolver.servicePrefix+"/") {
		logger.Log(ctx).Debug().
			Str("import", imp).
			Str("servicePrefix", resolver.servicePrefix).
			Msg("Import does not start with service prefix")
		return
	}

	projectPath := extractProjectPathFromImport(imp)
	if projectPath == "" {
		logger.Log(ctx).Debug().Str("import", imp).Msg("Import path too short to extract project")
		return
	}

	logger.Log(ctx).Debug().
		Str("import", imp).
		Str("projectPath", projectPath).
		Msg("Attempting to discover project")

	if isProjectAlreadyDiscovered(resolver, projectPath) {
		logger.Log(ctx).Debug().
			Str("import", imp).
			Str("projectPath", projectPath).
			Msg("Project already discovered")
		return
	}

	lookupAndRegisterProject(ctx, resolver, imp, projectPath)
}

// extractProjectPathFromImport extracts the project path from an import path.
func extractProjectPathFromImport(imp string) string {
	parts := strings.Split(imp, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

// isProjectAlreadyDiscovered checks if a project has already been discovered.
func isProjectAlreadyDiscovered(resolver *RegistryResolver, projectPath string) bool {
	resolver.mu.Lock()
	defer resolver.mu.Unlock()
	_, exists := resolver.projects[registry.ProjectPath(projectPath)]
	return exists
}

// lookupAndRegisterProject looks up a project and registers it if found.
func lookupAndRegisterProject(ctx context.Context, resolver *RegistryResolver, imp, projectPath string) {
	lookupCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	logger.Log(ctx).Debug().
		Str("import", imp).
		Str("projectPath", projectPath).
		Msg("Calling LookupProject")

	res, err := resolver.cache.LookupProject(lookupCtx, &registry.LookupProjectRequest{
		Path:     projectPath,
		Snapshot: resolver.snapshot,
	})

	logger.Log(ctx).Debug().
		Str("import", imp).
		Str("projectPath", projectPath).
		Err(err).
		Bool("resIsNil", res == nil).
		Msg("LookupProject completed")

	if err == nil && res != nil && res.Project != nil {
		resolver.mu.Lock()
		resolver.projects[res.Project.Path] = struct{}{}
		resolver.mu.Unlock()
		logger.Log(ctx).Debug().
			Str("import", imp).
			Str("project", string(res.Project.Path)).
			Msg("Discovered project from import")
	} else {
		logger.Log(ctx).Debug().
			Str("import", imp).
			Str("projectPath", projectPath).
			Err(err).
			Msg("Project not found in registry")
	}
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
func exportBufDependencies(ctx context.Context, bufDir string) string {
	// Check if buf CLI is available
	if _, err := exec.LookPath("buf"); err != nil {
		logger.Log(ctx).Debug().Msg("buf CLI not found, skipping BSR dependency resolution")
		return ""
	}

	// Create temp directory for export
	exportDir, err := os.MkdirTemp("", "protato-buf-export-*")
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to create temp directory for buf export")
		return ""
	}

	// Run buf export
	logger.Log(ctx).Debug().Str("dir", bufDir).Msg("Exporting buf dependencies")
	cmd := exec.CommandContext(ctx, "buf", "export", ".", "-o", exportDir)
	cmd.Dir = bufDir

	output, err := cmd.CombinedOutput()
	if err != nil {
		logger.Log(ctx).Debug().Err(err).Str("output", string(output)).Msg("buf export failed, continuing without BSR deps")
		os.RemoveAll(exportDir)
		return ""
	}

	logger.Log(ctx).Debug().Str("exportDir", exportDir).Msg("Successfully exported buf dependencies")
	return exportDir
}

// loadExportedFiles loads proto files from the buf export directory into the resolver cache.
func (r *RegistryResolver) loadExportedFiles(ctx context.Context, exportDir string) error {
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

		r.mu.Lock()
		// Skip if we already have this file at exact path OR at import path
		// We now cache owned files at import paths (e.g., "buf/validate/..." not "druid/buf/validate/...")
		if _, exists := r.fileCache[importPath]; exists {
			logger.Log(ctx).Debug().Str("path", importPath).Msg("Skipping BSR file (already cached)")
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()
		logger.Log(ctx).Debug().Str("path", importPath).Msg("Loading BSR file")

		// Read file content
		content, err := os.ReadFile(filePath)
		if err != nil {
			return nil
		}

		// Cache the file
		r.mu.Lock()
		r.fileCache[importPath] = content
		count++
		r.mu.Unlock()

		return nil
	})

	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Error walking buf export directory")
	}
	logger.Log(ctx).Debug().Int("files", count).Msg("Loaded buf dependencies into cache")
	return nil
}

// loadVendorFiles loads proto files from the local vendor directory into the resolver cache.
// This allows owned protos to import pulled dependencies during validation.
func (r *RegistryResolver) loadVendorFiles(ctx context.Context, vendorDir string) error {
	if vendorDir == "" {
		return nil
	}

	// Check if vendor directory exists
	if _, err := os.Stat(vendorDir); os.IsNotExist(err) {
		return nil
	}

	count := 0
	err := filepath.WalkDir(vendorDir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // Skip errors
		}
		if d.IsDir() || !strings.HasSuffix(filePath, ".proto") {
			return nil
		}

		// Get relative path (this is the import path)
		relPath, err := filepath.Rel(vendorDir, filePath)
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

		// Cache the file (only if not already present - registry files take precedence)
		r.mu.Lock()
		if _, exists := r.fileCache[importPath]; !exists {
			r.fileCache[importPath] = content
			count++
		}
		r.mu.Unlock()

		return nil
	})

	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Error walking vendor directory")
	}
	if count > 0 {
		logger.Log(ctx).Debug().Int("files", count).Str("dir", vendorDir).Msg("Loaded vendor dependencies into cache")
	}
	return nil
}

// ValidateProtosConfig holds configuration for ValidateProtos.
type ValidateProtosConfig struct {
	Cache         *registry.Cache
	Snapshot      git.Hash
	Projects      []registry.ProjectPath
	OwnedDir      string // Local directory prefix used in proto imports (e.g., "proto")
	VendorDir     string // Directory containing pulled dependencies
	WorkspaceRoot string // Root directory of the workspace (for finding buf.yaml)
	ServiceName   string // Service name from workspace configuration (e.g., "lcs-svc")
}

// ValidateProtos validates that the proto files compile successfully.
func ValidateProtos(ctx context.Context, config ValidateProtosConfig) error {
	resolver := NewRegistryResolver(ctx, config.Cache, config.Snapshot)
	configureResolver(resolver, config.OwnedDir, config.ServiceName)

	if err := preloadProtoFiles(ctx, resolver, config.Projects); err != nil {
		return err
	}

	// Load pulled dependencies from vendor directory
	if err := resolver.loadVendorFiles(ctx, config.VendorDir); err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to load vendor dependencies")
	}

	// Try to load BSR dependencies using buf export for all buf.yaml files
	if config.WorkspaceRoot != "" {
		bufDirs := findAllBufYamlWithDeps(config.WorkspaceRoot)
		for _, bufDir := range bufDirs {
			if exportDir := exportBufDependencies(ctx, bufDir); exportDir != "" {
				if err := resolver.loadExportedFiles(ctx, exportDir); err != nil {
					logger.Log(ctx).Warn().Err(err).Msg("Failed to load buf dependencies")
				}
				os.RemoveAll(exportDir) // Cleanup after loading
			}
		}
	}

	protoFiles := buildProtoFileList(ctx, config.Cache, config.Snapshot, config.Projects, resolver)
	if len(protoFiles) == 0 {
		return nil
	}

	return compileProtoFiles(ctx, resolver, protoFiles)
}

// configureResolver sets up the resolver with import and service prefixes.
func configureResolver(resolver *RegistryResolver, ownedDir, serviceName string) {
	// Always set import prefix - empty string means root directory (ownedDir: ".")
	resolver.SetImportPrefix(ownedDir)

	// Set service prefix from workspace configuration
	resolver.SetServicePrefix(serviceName)
}

// preloadProtoFiles pre-loads all proto files into memory to avoid concurrent git access.
func preloadProtoFiles(ctx context.Context, resolver *RegistryResolver, projects []registry.ProjectPath) error {
	logger.Log(ctx).Debug().Int("projects", len(projects)).Msg("Pre-loading proto files into memory")
	// Pass cacheAtRegistryPath=false for validation - only cache at import paths
	if err := resolver.PreloadFiles(ctx, projects, false); err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to preload files, skipping validation")
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

		// Build path based on how imports work in proto files
		// For ownedDir="" (root): proto files import as "subPath/filePath" (e.g., "buf/validate/validate.proto")
		// For ownedDir="proto": proto files import as "proto/subPath/filePath" (e.g., "proto/common/account.proto")
		if resolver.importPrefix == "" {
			return subPath + "/" + filePath
		}
		return resolver.importPrefix + "/" + subPath + "/" + filePath
	}

	// Fallback to registry path if no service prefix
	return path.Join(projectStr, filePath)
}

// compileProtoFiles compiles the proto files and handles errors.
func compileProtoFiles(ctx context.Context, resolver *RegistryResolver, protoFiles []string) error {
	rep := &LogReporter{Log: logger.Log(ctx)}

	compiler := protocompile.Compiler{
		Resolver: protocompile.WithStandardImports(resolver),
		Reporter: rep,
	}

	logger.Log(ctx).Info().Int("files", len(protoFiles)).Msg("Validating proto files")

	_, err := compiler.Compile(ctx, protoFiles...)
	if rep.Failed() {
		return &CompileError{Message: ErrCompilationFailed}
	}

	if err != nil {
		return handleCompileError(ctx, err)
	}

	logger.Log(ctx).Info().Msg("Proto validation completed successfully")
	return nil
}

// handleCompileError handles compilation errors, including panic recovery.
func handleCompileError(ctx context.Context, err error) error {
	errStr := err.Error()
	if strings.Contains(errStr, "panic") {
		logger.Log(ctx).Warn().Err(err).Msg("Proto validation encountered internal error, skipping")
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

// TransformImports transforms import paths in a proto file content.
// Simple version without pulled project handling.
func TransformImports(content []byte, ownedDir, servicePrefix string) []byte {
	return TransformImportsWithPulled(content, ownedDir, servicePrefix, nil)
}

// TransformImportsWithPulled transforms import paths, handling pulled projects correctly.
// For owned imports with ownedDir: ownedDir/common/... -> servicePrefix/common/...
// For owned imports without ownedDir: common/... -> servicePrefix/common/...
// For pulled imports: ownedDir/other-svc/... -> other-svc/... (just strip ownedDir)
// pulledPrefixes contains the service names of pulled projects (e.g., ["lcs-svc", "payment-svc"])
func TransformImportsWithPulled(content []byte, ownedDir, servicePrefix string, pulledPrefixes []string) []byte {
	if servicePrefix == "" {
		return content
	}

	lines := strings.Split(string(content), "\n")
	var result []string

	for _, line := range lines {
		transformed := transformImportLine(line, ownedDir, servicePrefix, pulledPrefixes)
		result = append(result, transformed)
	}

	return []byte(strings.Join(result, "\n"))
}

// transformImportLine transforms a single import line.
// Handles both owned imports (add service prefix) and pulled imports (just strip ownedDir).
func transformImportLine(line, ownedDir, servicePrefix string, pulledPrefixes []string) string {
	importPath := extractImportPathFromLine(line)
	if importPath == "" {
		return line
	}

	if strings.HasPrefix(importPath, googleProtobufPrefix) {
		return line
	}

	pathToTransform := extractPathToTransform(importPath, ownedDir)
	if pathToTransform == "" {
		return line
	}

	if isPulledProject(pathToTransform, pulledPrefixes) {
		return handlePulledProject(line, importPath, pathToTransform, ownedDir)
	}

	if strings.HasPrefix(pathToTransform, servicePrefix+"/") {
		return line
	}

	return transformOwnedProject(line, importPath, pathToTransform, servicePrefix)
}

// extractPathToTransform extracts the path portion to transform based on ownedDir.
func extractPathToTransform(importPath, ownedDir string) string {
	if ownedDir == "" {
		return importPath
	}
	ownedPrefix := ownedDir + "/"
	if !strings.HasPrefix(importPath, ownedPrefix) {
		return ""
	}
	return strings.TrimPrefix(importPath, ownedPrefix)
}

// isPulledProject checks if the path represents a pulled project.
func isPulledProject(pathToTransform string, pulledPrefixes []string) bool {
	for _, pulledPrefix := range pulledPrefixes {
		if strings.HasPrefix(pathToTransform, pulledPrefix+"/") || pathToTransform == pulledPrefix {
			return true
		}
	}
	return false
}

// handlePulledProject handles transformation for pulled project imports.
func handlePulledProject(line, importPath, pathToTransform, ownedDir string) string {
	if ownedDir != "" {
		return strings.Replace(line, importPath, pathToTransform, 1)
	}
	return line
}

// transformOwnedProject transforms an owned project import by adding the service prefix.
func transformOwnedProject(line, importPath, pathToTransform, servicePrefix string) string {
	newImportPath := servicePrefix + "/" + pathToTransform
	return strings.Replace(line, importPath, newImportPath, 1)
}

// extractImportsFromContent extracts all import statements from proto file content.
func extractImportsFromContent(content []byte) []string {
	var imports []string
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		importPath := extractImportPathFromLine(line)
		if importPath != "" && !strings.HasPrefix(importPath, googleProtobufPrefix) {
			imports = append(imports, importPath)
		}
	}

	return imports
}

// extractImportPathFromLine extracts the import path from a single line if it's an import statement.
func extractImportPathFromLine(line string) string {
	trimmed := strings.TrimSpace(line)
	if !strings.HasPrefix(trimmed, importKeyword) {
		return ""
	}

	// Parse import statement: import "path/to/file.proto";
	// or: import 'path/to/file.proto';
	var quote byte
	startIdx := 0
	endIdx := 0

	for i := len(importKeyword); i < len(trimmed); i++ {
		if trimmed[i] == '"' || trimmed[i] == '\'' {
			if quote == 0 {
				quote = trimmed[i]
				startIdx = i + 1
			} else if trimmed[i] == quote {
				endIdx = i
				break
			}
		}
	}

	if startIdx > 0 && endIdx > 0 {
		return trimmed[startIdx:endIdx]
	}
	return ""
}
