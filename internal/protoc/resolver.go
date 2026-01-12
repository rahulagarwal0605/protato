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
// If cacheAtRegistryPath is true, files are cached at both registry paths and import paths.
// This is needed for dependency discovery where files are compiled using registry paths.
func (r *RegistryResolver) PreloadFiles(ctx context.Context, projects []registry.ProjectPath, cacheAtRegistryPath bool) error {
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
			// Cache at import path ONLY (how files are imported in proto files)
			// Don't cache at registryPath to avoid duplicates
			if r.servicePrefix != "" && strings.HasPrefix(registryPath, r.servicePrefix+"/") {
				subPath := strings.TrimPrefix(registryPath, r.servicePrefix+"/")

				// Skip google/protobuf - those come from standard imports
				if strings.Contains(subPath, "google/protobuf/") {
					r.projects[project] = struct{}{}
					r.mu.Unlock()
					continue
				}

				// Cache at import path (how files are imported in proto files)
				// For ownedDir="": import path is subPath (e.g., "buf/validate/validate.proto")
				// For ownedDir="proto": import path is proto/subPath (e.g., "proto/common/account.proto")
				var cachePath string
				if r.importPrefix != "" {
					cachePath = r.importPrefix + "/" + subPath
				} else {
					cachePath = subPath
				}

				// Un-transform imports for import path cache (matches local import paths)
				// Registry content has: import "druid/buf/validate/..."
				// We need:              import "buf/validate/..."
				untransformedContent := r.untransformImports(content)
				r.fileCache[cachePath] = untransformedContent

				// Also cache at registry path if requested (for dependency discovery)
				// Keep transformed imports for registry path (matches registry import paths)
				if cacheAtRegistryPath {
					r.fileCache[registryPath] = content // Keep original content with transformed imports
					log.Debug().Str("registryPath", registryPath).Str("cachePath", cachePath).Msg("Cached file at both paths")
				} else {
					log.Debug().Str("registryPath", registryPath).Str("cachePath", cachePath).Msg("Cached file")
				}
			} else {
				// Fallback: cache at registry path for projects without service prefix
				r.fileCache[registryPath] = content
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
	// Safety check
	if r == nil {
		return protocompile.SearchResult{}, fmt.Errorf("resolver is nil")
	}

	ctx := context.Background()
	log := logger.Log(ctx)

	log.Debug().
		Str("filePath", filePath).
		Bool("preloaded", r.preloaded).
		Msg("FindFileByPath: called")

	// Map import path first to ensure consistency
	// e.g., "buf/validate/..." -> "druid/buf/validate/..." when ownedDir is "."
	mappedPath := r.mapImportPath(filePath)

	// Check cache for mapped path first
	r.mu.Lock()
	cached, ok := r.fileCache[mappedPath]
	r.mu.Unlock()

	if ok {
		if cached == nil {
			log.Error().
				Str("filePath", filePath).
				Str("mappedPath", mappedPath).
				Msg("FindFileByPath: cached content is nil")
			return protocompile.SearchResult{}, fmt.Errorf("cached content is nil for %s", filePath)
		}
		log.Debug().
			Str("filePath", filePath).
			Str("mappedPath", mappedPath).
			Int("size", len(cached)).
			Msg("FindFileByPath: found in cache (mapped)")
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
				log.Error().
					Str("filePath", filePath).
					Msg("FindFileByPath: cached content is nil (original)")
				return protocompile.SearchResult{}, fmt.Errorf("cached content is nil for %s", filePath)
			}
			log.Debug().
				Str("filePath", filePath).
				Int("size", len(cached)).
				Msg("FindFileByPath: found in cache (original)")
			return protocompile.SearchResult{
				Source: bytes.NewReader(cached),
			}, nil
		}
	}

	// If preloaded, file not found in cache means it doesn't exist
	if r.preloaded {
		log.Debug().
			Str("filePath", filePath).
			Str("mappedPath", mappedPath).
			Msg("FindFileByPath: not found (preloaded mode)")
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Debug: log when falling back to git
	log.Debug().
		Str("filePath", filePath).
		Str("mappedPath", mappedPath).
		Bool("preloaded", r.preloaded).
		Msg("FindFileByPath: falling back to git")

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

	log := logger.Log(ctx)

	// Use original path for project lookup (don't map - we need the full registry path)
	// e.g., "lcs-svc/vendors/buf/validate/validate.proto" should lookup project "lcs-svc/vendors/buf/validate"
	log.Debug().Str("filePath", filePath).Msg("loadFileFromGit: looking up project")

	res, err := r.cache.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     filePath,
		Snapshot: r.snapshot,
	})
	if err != nil {
		log.Debug().Err(err).Str("filePath", filePath).Msg("loadFileFromGit: lookup failed")
		return protocompile.SearchResult{}, err
	}

	if res == nil || res.Project == nil {
		log.Debug().Str("filePath", filePath).Msg("loadFileFromGit: project not found")
		return protocompile.SearchResult{}, registry.ErrNotFound
	}

	// Record discovered project
	r.mu.Lock()
	r.projects[res.Project.Path] = struct{}{}
	r.mu.Unlock()
	log.Debug().Str("filePath", filePath).Str("project", string(res.Project.Path)).Msg("loadFileFromGit: discovered project")

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
	var result []string
	changed := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			result = append(result, line)
			continue
		}

		// Find the import path (between quotes)
		var quote byte
		var startIdx, endIdx int
		for i := 7; i < len(trimmed); i++ {
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

		if startIdx == 0 || endIdx == 0 {
			result = append(result, line)
			continue
		}

		importPath := trimmed[startIdx:endIdx]

		// Check if import has service prefix
		if strings.HasPrefix(importPath, r.servicePrefix+"/") {
			subPath := strings.TrimPrefix(importPath, r.servicePrefix+"/")
			var newImportPath string
			if r.importPrefix != "" {
				newImportPath = r.importPrefix + "/" + subPath
			} else {
				newImportPath = subPath
			}
			newLine := strings.Replace(line, importPath, newImportPath, 1)
			result = append(result, newLine)
			changed = true
		} else {
			result = append(result, line)
		}
	}

	if changed {
		return []byte(strings.Join(result, "\n"))
	}
	return content
}

func (r *RegistryResolver) mapImportPath(importPath string) string {
	if r.servicePrefix == "" {
		return importPath
	}

	// Skip standard imports (google/protobuf) - they're provided by protocompile
	if strings.HasPrefix(importPath, "google/protobuf/") {
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

	// Extract service prefix from project paths
	if len(projects) > 0 {
		projectPath := string(projects[0])
		if idx := strings.Index(projectPath, "/"); idx > 0 {
			resolver.SetServicePrefix(projectPath[:idx])
		}
	}

	// Get all proto files from the requested projects
	// Build import paths (not registry paths) for compilation
	// Files will be cached at both registry and import paths during PreloadFiles
	var protoFiles []string
	for _, project := range projects {
		filesRes, err := cache.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			return nil, err
		}

		// Extract service prefix if not already set
		servicePrefix := resolver.servicePrefix
		if servicePrefix == "" {
			projectStr := string(project)
			if idx := strings.Index(projectStr, "/"); idx > 0 {
				servicePrefix = projectStr[:idx]
				resolver.SetServicePrefix(servicePrefix)
			}
		}

		for _, f := range filesRes.Files {
			// Build import path (how files are imported in proto files)
			// For dependency discovery, importPrefix is empty, so use subPath directly
			projectStr := string(project)
			if servicePrefix != "" && strings.HasPrefix(projectStr, servicePrefix+"/") {
				subPath := strings.TrimPrefix(projectStr, servicePrefix+"/")
				importPath := path.Join(subPath, f.Path)
				protoFiles = append(protoFiles, importPath)
			} else {
				// Fallback to registry path
				protoFiles = append(protoFiles, path.Join(projectStr, f.Path))
			}
		}

		// Mark requested projects as discovered
		resolver.mu.Lock()
		resolver.projects[project] = struct{}{}
		resolver.mu.Unlock()
	}

	if len(protoFiles) == 0 {
		logger.Log(ctx).Debug().Msg("No proto files to compile for dependency discovery")
		return projects, nil
	}

	logger.Log(ctx).Debug().Int("count", len(protoFiles)).Strs("files", protoFiles).Msg("Compiling files for dependency discovery")

	// Preload the initial project files so protocompile can read them and discover imports
	// This ensures that when protocompile tries to compile these files, they're already in the cache
	// Files will be cached at registry paths (e.g., "lcs-svc/customeraccounts/accounts.proto")
	// since registry files have registry paths in their imports
	// Pass cacheAtRegistryPath=true so files are cached at both registry and import paths
	if err := resolver.PreloadFiles(ctx, projects, true); err != nil {
		logger.Log(ctx).Debug().Err(err).Msg("Failed to preload files for dependency discovery")
		// Continue anyway - files will be loaded on-demand
	}

	// Reset preloaded flag to allow on-demand loading of dependencies
	// We want to discover new projects when imports are resolved
	resolver.mu.Lock()
	resolver.preloaded = false
	// Log cache contents for debugging
	logger.Log(ctx).Debug().
		Int("cachedFiles", len(resolver.fileCache)).
		Msg("Cache contents before compilation")
	for path := range resolver.fileCache {
		logger.Log(ctx).Debug().Str("cachedPath", path).Msg("Cached file path")
	}
	resolver.mu.Unlock()

	// Parse proto files directly to extract imports instead of compiling
	// This avoids panics in protocompile and is more reliable for dependency discovery
	logger.Log(ctx).Debug().Strs("files", protoFiles).Msg("Parsing proto files for dependency discovery")

	// Parse each file to extract imports
	// Use registry paths to get content with transformed imports
	for _, protoFile := range protoFiles {
		// Try to get content from registry path first (has transformed imports)
		// Registry path format: lcs-svc/customeraccounts/accounts.proto
		var content []byte
		var ok bool

		resolver.mu.Lock()
		// Try registry path first (servicePrefix/protoFile)
		if resolver.servicePrefix != "" {
			registryPath := resolver.servicePrefix + "/" + protoFile
			content, ok = resolver.fileCache[registryPath]
		}
		// Fallback to import path
		if !ok {
			content, ok = resolver.fileCache[protoFile]
		}
		resolver.mu.Unlock()

		if !ok {
			logger.Log(ctx).Debug().Str("file", protoFile).Msg("File not found in cache, skipping")
			continue
		}

		// Extract imports from file content
		// Registry files have transformed imports like: import "lcs-svc/vendors/google/api/annotations.proto";
		imports := extractImportsFromContent(content)
		logger.Log(ctx).Debug().Str("file", protoFile).Int("importCount", len(imports)).Msg("Extracted imports from file")

		for _, imp := range imports {
			logger.Log(ctx).Debug().Str("file", protoFile).Str("import", imp).Msg("Found import")

			// Skip standard imports
			if strings.HasPrefix(imp, "google/protobuf/") {
				continue
			}

			// Directly discover project from import path instead of calling FindFileByPath
			// Extract project path from import: "lcs-svc/vendors/google/api/annotations.proto" -> "lcs-svc/vendors/google/api"
			if resolver.servicePrefix != "" && strings.HasPrefix(imp, resolver.servicePrefix+"/") {
				// Extract project path (everything before the last "/" + filename)
				parts := strings.Split(imp, "/")
				if len(parts) >= 2 {
					// Project path is everything except the filename
					projectPath := strings.Join(parts[:len(parts)-1], "/")

					logger.Log(ctx).Debug().
						Str("import", imp).
						Str("projectPath", projectPath).
						Msg("Attempting to discover project")

					// Check if we've already discovered this project
					resolver.mu.Lock()
					_, exists := resolver.projects[registry.ProjectPath(projectPath)]
					resolver.mu.Unlock()

					if !exists {
						// Try to lookup the project to verify it exists
						// Use a timeout context to avoid hanging
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
					} else {
						logger.Log(ctx).Debug().
							Str("import", imp).
							Str("projectPath", projectPath).
							Msg("Project already discovered")
					}
				} else {
					logger.Log(ctx).Debug().
						Str("import", imp).
						Msg("Import path too short to extract project")
				}
			} else {
				logger.Log(ctx).Debug().
					Str("import", imp).
					Str("servicePrefix", resolver.servicePrefix).
					Msg("Import does not start with service prefix")
			}
		}
	}

	logger.Log(ctx).Debug().Int("discovered", len(resolver.projects)).Msg("Dependency discovery complete")

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

		r.mu.Lock()
		// Skip if we already have this file at exact path OR at import path
		// We now cache owned files at import paths (e.g., "buf/validate/..." not "druid/buf/validate/...")
		if _, exists := r.fileCache[importPath]; exists {
			log.Debug().Str("path", importPath).Msg("Skipping BSR file (already cached)")
			r.mu.Unlock()
			return nil
		}
		r.mu.Unlock()
		log.Debug().Str("path", importPath).Msg("Loading BSR file")

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
		log.Warn().Err(err).Msg("Error walking buf export directory")
	}
	log.Debug().Int("files", count).Msg("Loaded buf dependencies into cache")
	return nil
}

// loadVendorFiles loads proto files from the local vendor directory into the resolver cache.
// This allows owned protos to import pulled dependencies during validation.
func (r *RegistryResolver) loadVendorFiles(vendorDir string, log *zerolog.Logger) error {
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
		log.Warn().Err(err).Msg("Error walking vendor directory")
	}
	if count > 0 {
		log.Debug().Int("files", count).Str("dir", vendorDir).Msg("Loaded vendor dependencies into cache")
	}
	return nil
}

// ValidateProtos validates that the proto files compile successfully.
// ownedDir is the local directory prefix used in proto imports (e.g., "proto").
// workspaceRoot is the root directory of the workspace (for finding buf.yaml).
// vendorDir is the directory containing pulled dependencies.
func ValidateProtos(
	ctx context.Context,
	cache *registry.Cache,
	snapshot git.Hash,
	projects []registry.ProjectPath,
	ownedDir string,
	vendorDir string,
	workspaceRoot string,
) error {
	log := logger.Log(ctx)

	resolver := NewRegistryResolver(ctx, cache, snapshot)
	configureResolver(resolver, projects, ownedDir, log)

	if err := preloadProtoFiles(ctx, resolver, projects, log); err != nil {
		return err
	}

	// Load pulled dependencies from vendor directory
	if err := resolver.loadVendorFiles(vendorDir, log); err != nil {
		log.Warn().Err(err).Msg("Failed to load vendor dependencies")
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
	// Pass cacheAtRegistryPath=false for validation - only cache at import paths
	if err := resolver.PreloadFiles(ctx, projects, false); err != nil {
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
	trimmed := strings.TrimSpace(line)

	// Check if this is an import line
	if !strings.HasPrefix(trimmed, "import ") {
		return line
	}

	// Find the import path (between quotes)
	var quote byte
	var startIdx, endIdx int

	for i := 7; i < len(trimmed); i++ { // Start after "import "
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

	if startIdx == 0 || endIdx == 0 {
		return line // Couldn't parse, return unchanged
	}

	importPath := trimmed[startIdx:endIdx]

	// Skip standard imports (google/protobuf) - they're provided by protocompile
	if strings.HasPrefix(importPath, "google/protobuf/") {
		return line
	}

	// Determine the path to transform
	var pathToTransform string

	if ownedDir == "" {
		// No ownedDir prefix - transform the entire import path
		pathToTransform = importPath
	} else {
		// Check if import starts with ownedDir
		ownedPrefix := ownedDir + "/"
		if !strings.HasPrefix(importPath, ownedPrefix) {
			return line // Not prefixed with ownedDir, return unchanged
		}
		// Get the path after ownedDir
		pathToTransform = strings.TrimPrefix(importPath, ownedPrefix)
	}

	// Check if this is a pulled project import (already has a service prefix)
	for _, pulledPrefix := range pulledPrefixes {
		if strings.HasPrefix(pathToTransform, pulledPrefix+"/") || pathToTransform == pulledPrefix {
			// This is a pulled project - use the path without ownedDir prefix
			if ownedDir != "" {
				newLine := strings.Replace(line, importPath, pathToTransform, 1)
				return newLine
			}
			// If no ownedDir, it's already correct
			return line
		}
	}

	// Check if import already has the service prefix (avoid double-prefixing)
	if strings.HasPrefix(pathToTransform, servicePrefix+"/") {
		return line
	}

	// This is an owned project - add service prefix
	// common/... -> servicePrefix/common/...
	newImportPath := servicePrefix + "/" + pathToTransform
	newLine := strings.Replace(line, importPath, newImportPath, 1)
	return newLine
}

// extractImportsFromContent extracts all import statements from proto file content.
func extractImportsFromContent(content []byte) []string {
	var imports []string
	lines := strings.Split(string(content), "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "import ") {
			continue
		}

		// Parse import statement: import "path/to/file.proto";
		// or: import 'path/to/file.proto';
		var quote byte
		startIdx := 0
		endIdx := 0

		for i := 7; i < len(trimmed); i++ { // Start after "import "
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
			importPath := trimmed[startIdx:endIdx]
			// Skip standard imports
			if !strings.HasPrefix(importPath, "google/protobuf/") {
				imports = append(imports, importPath)
			}
		}
	}

	return imports
}
