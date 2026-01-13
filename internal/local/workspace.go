package local

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/utils"
)

const (
	configFileName    = "protato.yaml"
	lockFileName      = "protato.lock"
	gitattributesName = ".gitattributes"
	protoFileExt      = ".proto"
)

// Workspace represents a local protato workspace.
type Workspace struct {
	root   string  // Repository root directory
	config *Config // Loaded configuration
}

// Init initializes a new workspace.
func Init(ctx context.Context, root string, config *Config, force bool) (*Workspace, error) {
	configPath := configPath(root)

	// Check if already initialized
	if _, err := os.Stat(configPath); err == nil && !force {
		return nil, ErrAlreadyInitialized
	}

	// Build config (merge with existing if force, otherwise use provided)
	finalConfig, err := buildConfig(configPath, config, force)
	if err != nil {
		return nil, err
	}

	// Write config file
	if err := writeConfig(configPath, finalConfig); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Create directories
	ownedDir, err := finalConfig.OwnedDir()
	if err != nil {
		return nil, fmt.Errorf("get owned directory: %w", err)
	}
	vendorDir, err := finalConfig.VendorDir()
	if err != nil {
		return nil, fmt.Errorf("get vendor directory: %w", err)
	}
	if err := utils.CreateDir(filepath.Join(root, ownedDir), "owned protos"); err != nil {
		return nil, err
	}
	if err := utils.CreateDir(filepath.Join(root, vendorDir), "vendor protos"); err != nil {
		return nil, err
	}

	return &Workspace{
		root:   root,
		config: finalConfig,
	}, nil
}

// buildConfig creates or merges configuration based on force flag.
func buildConfig(configPath string, config *Config, force bool) (*Config, error) {
	if force {
		return buildConfigWithMerge(configPath, config)
	}
	return config, nil
}

// buildConfigWithMerge merges config with existing config if it exists, otherwise uses provided config.
func buildConfigWithMerge(configPath string, config *Config) (*Config, error) {
	existingConfig, err := readConfig(configPath)
	if err != nil {
		// No existing config, use provided config
		return config, nil
	}

	// Merge with existing config
	return mergeConfig(existingConfig, config), nil
}

// mergeConfig merges new config into existing config.
func mergeConfig(existing *Config, new *Config) *Config {
	config := existing

	// Update service if provided
	if new.Service != "" {
		config.Service = new.Service
	}

	// Update directories if provided
	if new.Directories.Owned != "" {
		config.Directories.Owned = new.Directories.Owned
	}
	if new.Directories.Vendor != "" {
		config.Directories.Vendor = new.Directories.Vendor
	}

	// Merge projects if provided
	if len(new.Projects) > 0 {
		config.Projects = utils.MergeStringSlice(config.Projects, new.Projects)
	}

	// Auto-discover wins - use the provided value
	config.AutoDiscover = new.AutoDiscover

	// Merge ignores if provided
	if len(new.Ignores) > 0 {
		config.Ignores = utils.MergeStringSlice(config.Ignores, new.Ignores)
	}

	return config
}

// Open opens an existing workspace.
func Open(ctx context.Context, root string) (*Workspace, error) {
	configPath := configPath(root)

	// Check if initialized
	if utils.DirNotExists(configPath) {
		return nil, ErrNotInitialized
	}

	// Read config
	config, err := readConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return &Workspace{
		root:   root,
		config: config,
	}, nil
}

// Root returns the workspace root directory.
func (ws *Workspace) Root() string {
	return ws.root
}

// getDirPath returns the absolute directory path for a given directory getter function.
func (ws *Workspace) getDirPath(getDir func() (string, error), dirName string) (string, error) {
	dir, err := getDir()
	if err != nil {
		return "", fmt.Errorf("get %s directory: %w", dirName, err)
	}
	return filepath.Join(ws.root, dir), nil
}

// projectPathJoin joins a directory with a project path.
func projectPathJoin(dir string, project ProjectPath) string {
	return filepath.Join(dir, string(project))
}

// lockFilePath returns the path to a lock file for a project.
func lockFilePath(projectDir, projectPath string) string {
	return filepath.Join(projectDir, projectPath, lockFileName)
}

// configPath returns the path to the config file in the given root directory.
func configPath(root string) string {
	return filepath.Join(root, configFileName)
}

// OwnedDir returns the absolute directory path for owned (producer) protos.
func (ws *Workspace) OwnedDir() (string, error) {
	return ws.getDirPath(ws.config.OwnedDir, "owned")
}

// OwnedDirName returns just the owned directory name (e.g., "proto") without the root path.
func (ws *Workspace) OwnedDirName() (string, error) {
	return ws.config.OwnedDir()
}

// VendorDir returns the directory path for consumed (vendor) protos.
func (ws *Workspace) VendorDir() (string, error) {
	return ws.getDirPath(ws.config.VendorDir, "vendor")
}

// ServiceName returns the service name for registry namespacing.
func (ws *Workspace) ServiceName() string {
	if ws.config != nil {
		return ws.config.Service
	}
	return ""
}

// RegistryProjectPath returns the full registry path for a local project.
// It prefixes the project path with the service name.
func (ws *Workspace) RegistryProjectPath(localProject ProjectPath) (ProjectPath, error) {
	if ws.config == nil || ws.config.Service == "" {
		return "", ErrServiceNotConfigured
	}
	return ProjectPath(utils.BuildServicePrefixedPath(ws.config.Service, string(localProject))), nil
}

// LocalProjectPath converts a registry project path to a local project path.
// It strips the service name prefix if it matches.
func (ws *Workspace) LocalProjectPath(registryProject ProjectPath) ProjectPath {
	if ws.config != nil && ws.config.Service != "" {
		prefix := ws.config.Service + "/"
		if strings.HasPrefix(string(registryProject), prefix) {
			return ProjectPath(strings.TrimPrefix(string(registryProject), prefix))
		}
	}
	return registryProject
}

// OwnedProjects returns the list of owned projects.
// All projects must be within the owned directory. Projects and ignores patterns are applied within the owned directory.
// When auto_discover=true: discovers all projects in owned dir, then filters by ignores
// When auto_discover=false: finds projects matching project patterns, then filters by ignores
func (ws *Workspace) OwnedProjects() ([]ProjectPath, error) {
	var projects []ProjectPath
	var err error

	if ws.config.AutoDiscover {
		// Discover all projects (no pattern filter), but filter out pulled projects
		projects, err = ws.discoverProjects()
		if err != nil {
			return nil, err
		}
	} else {
		// Find projects matching project patterns
		projects, err = ws.discoverProjectsByPattern()
		if err != nil {
			return nil, err
		}
	}

	// Apply ignores: filter out projects matching ignore patterns
	projects = ws.applyProjectIgnores(projects)

	return projects, nil
}

// discoverProjects discovers all projects in the owned directory.
// Filters out pulled projects (projects with protato.lock).
func (ws *Workspace) discoverProjects() ([]ProjectPath, error) {
	return ws.scanProjects(nil)
}

// scanProjects scans the owned directory and finds projects.
// filterPattern: optional glob pattern to filter projects (nil = return all projects)
// Always filters out pulled projects (projects with protato.lock)
// Returns paths relative to the owned directory.
func (ws *Workspace) scanProjects(filterPattern *string) ([]ProjectPath, error) {
	ownedPath, err := ws.OwnedDir()
	if err != nil {
		return nil, err
	}

	if utils.DirNotExists(ownedPath) {
		return []ProjectPath{}, nil
	}

	var projects []ProjectPath
	seen := make(map[string]bool)

	err = filepath.WalkDir(ownedPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		projectPath := ws.processProtoFile(p, d, ownedPath, filterPattern, seen)
		if projectPath != "" {
			projects = append(projects, ProjectPath(projectPath))
		}
		return nil
	})

	return projects, err
}

// processProtoFile processes a proto file entry and returns the project path if valid.
func (ws *Workspace) processProtoFile(p string, d fs.DirEntry, ownedPath string, filterPattern *string, seen map[string]bool) string {
	if d.IsDir() || !strings.HasSuffix(d.Name(), protoFileExt) {
		return ""
	}

	protoDir := filepath.Dir(p)
	relToOwned, err := utils.RelPathToSlash(ownedPath, protoDir)
	if err != nil {
		return ""
	}

	if filterPattern != nil && !ws.matchesPattern(relToOwned, []string{*filterPattern}) {
		return ""
	}

	if seen[relToOwned] {
		return ""
	}
	seen[relToOwned] = true

	isPulled, err := ws.isPulledProject(relToOwned)
	if err != nil || isPulled {
		return ""
	}

	return relToOwned
}

// discoverProjectsByPattern finds projects matching project patterns within the owned directory.
// Only called when auto_discover=false.
// Project patterns are matched against paths relative to the owned directory.
// Deduplicates results since multiple patterns can match the same project.
func (ws *Workspace) discoverProjectsByPattern() ([]ProjectPath, error) {
	if len(ws.config.Projects) == 0 {
		return []ProjectPath{}, nil
	}

	// Find projects matching project patterns (searches within owned directory only)
	var allMatches []ProjectPath
	for _, projectPattern := range ws.config.Projects {
		matches, err := ws.scanProjects(&projectPattern)
		if err != nil {
			return nil, err
		}
		allMatches = append(allMatches, matches...)
	}

	// Deduplicate since multiple patterns can match the same project
	return utils.Deduplicate(allMatches, func(p ProjectPath) string { return string(p) }), nil
}

// applyProjectIgnores filters projects by ignore patterns.
// Ignore patterns are matched against project paths (relative to owned directory).
func (ws *Workspace) applyProjectIgnores(projects []ProjectPath) []ProjectPath {
	if len(ws.config.Ignores) == 0 {
		return projects
	}

	var filtered []ProjectPath
	for _, p := range projects {
		if !ws.matchesPattern(string(p), ws.config.Ignores) {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// applyFileIgnores filters files by ignore patterns.
// files: slice of files to filter
// project: project path relative to owned directory (e.g., "api/v1")
// Returns filtered slice of files that don't match ignore patterns.
func (ws *Workspace) applyFileIgnores(files []ProjectFile, project ProjectPath) []ProjectFile {
	if len(ws.config.Ignores) == 0 {
		return files
	}

	var filtered []ProjectFile
	for _, f := range files {
		// Construct full path (project/file) relative to owned directory
		fullPath := path.Join(string(project), f.Path)
		if !ws.matchesPattern(fullPath, ws.config.Ignores) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

// matchesPattern checks if a project path matches any pattern in the given list.
func (ws *Workspace) matchesPattern(projectPath string, patterns []string) bool {
	for _, pattern := range patterns {
		if utils.MatchPattern(pattern, projectPath) {
			return true
		}
	}
	return false
}

// isPulledProject checks if a project is a pulled project (has protato.lock file).
// This is used to distinguish owned vs pulled projects when both directories are the same.
func (ws *Workspace) isPulledProject(projectPath string) (bool, error) {
	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return false, err
	}
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return false, err
	}

	// Check in owned directory (for when owned == vendor)
	if utils.FileExists(lockFilePath(ownedDir, projectPath)) {
		return true, nil
	}

	// Also check in vendor directory if different
	if ownedDir != vendorDir {
		if utils.FileExists(lockFilePath(vendorDir, projectPath)) {
			return true, nil
		}
	}

	return false, nil
}

// ReceivedProjects returns the list of received (pulled) projects.
func (ws *Workspace) ReceivedProjects(ctx context.Context) ([]*ReceivedProject, error) {
	vendorPath, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	if utils.DirNotExists(vendorPath) {
		return []*ReceivedProject{}, nil
	}

	owned := ws.buildOwnedProjectsMap()
	return ws.findReceivedProjectsInVendor(ctx, vendorPath, owned)
}

// buildOwnedProjectsMap builds a map of owned project paths for filtering.
func (ws *Workspace) buildOwnedProjectsMap() map[string]bool {
	ownedProjects, err := ws.OwnedProjects()
	if err != nil {
		return make(map[string]bool)
	}
	owned := ws.projectPathsToMap(ownedProjects)
	// Also add service-prefixed paths
	if ws.config.Service != "" {
		for _, p := range ownedProjects {
			owned[utils.BuildServicePrefixedPath(ws.config.Service, string(p))] = true
		}
	}
	return owned
}

// findReceivedProjectsInVendor finds received projects in the vendor directory.
func (ws *Workspace) findReceivedProjectsInVendor(ctx context.Context, vendorPath string, owned map[string]bool) ([]*ReceivedProject, error) {
	var received []*ReceivedProject

	// Walk vendor directory looking for lock files
	err := filepath.WalkDir(vendorPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != lockFileName {
			return nil
		}

		// Get project path from lock file location
		projectPath, err := utils.RelPathToSlash(vendorPath, filepath.Dir(p))
		if err != nil {
			return nil
		}

		// Skip owned projects
		if owned[projectPath] {
			return nil
		}

		// Read lock file
		lock, err := readLockFile(p)
		if err != nil {
			logger.Log(ctx).Warn().Err(err).Str("path", p).Msg("Failed to read lock file")
			return nil
		}

		received = append(received, &ReceivedProject{
			Project:          ProjectPath(projectPath),
			ProviderSnapshot: lock.Snapshot,
		})

		return nil
	})

	return received, err
}

// AddOwnedProjects adds new owned projects to the configuration.
func (ws *Workspace) AddOwnedProjects(projects []string) error {
	// Add to existing projects
	existing := utils.StringSliceToMap(ws.config.Projects)

	for _, ps := range projects {
		if !existing[ps] {
			ws.config.Projects = append(ws.config.Projects, ps)
			existing[ps] = true
		}

		// Create project directory in owned directory
		ownedDir, err := ws.OwnedDir()
		if err != nil {
			return err
		}
		projectPath := filepath.Join(ownedDir, ps)
		if err := utils.CreateDir(projectPath, "project"); err != nil {
			return err
		}
	}

	// Write updated config
	return writeConfig(configPath(ws.root), ws.config)
}

// ReceiveProject starts receiving a project (into vendor directory).
func (ws *Workspace) ReceiveProject(req *ReceiveProjectRequest) (*ProjectReceiver, error) {
	// Received projects go into the vendor directory
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	projectRoot := projectPathJoin(vendorDir, req.Project)
	return &ProjectReceiver{
		ws:          ws,
		project:     req.Project,
		projectRoot: projectRoot,
		snapshot:    req.Snapshot,
	}, nil
}

// Write writes data to the file.
func (w *ProjectFileWriter) Write(p []byte) (int, error) {
	w.hash.Write(p)
	return w.file.Write(p)
}

// Close closes the file.
func (w *ProjectFileWriter) Close() error {
	err := w.file.Close()
	newHash := w.hash.Sum(nil)

	// Check if file changed
	changed := len(w.existingHash) == 0 || !utils.HashEqual(newHash, w.existingHash)
	w.onClose(changed)

	return err
}

// listProjectFiles lists files in a project directory.
func (ws *Workspace) listProjectFiles(projectPath string, project ProjectPath, applyIgnores bool) ([]ProjectFile, error) {
	var files []ProjectFile

	if utils.DirNotExists(projectPath) {
		return files, nil
	}

	err := filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only process .proto files (skip directories and non-proto files)
		if d.IsDir() || !strings.HasSuffix(d.Name(), protoFileExt) {
			return nil
		}

		// Get relative path
		relPath, err := utils.RelPathToSlash(projectPath, p)
		if err != nil {
			return nil
		}

		files = append(files, ProjectFile{
			Path:         relPath,
			AbsolutePath: p,
		})

		return nil
	})
	if err != nil {
		return files, err
	}

	// Apply ignores if requested
	if applyIgnores {
		files = ws.applyFileIgnores(files, project)
	}

	return files, nil
}

// ListOwnedProjectFiles lists all files in an owned project.
// project: path relative to the owned directory (e.g., "api/v1")
func (ws *Workspace) ListOwnedProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return nil, err
	}
	return ws.listProjectFiles(projectPathJoin(ownedDir, project), project, true)
}

// ListVendorProjectFiles lists all files in a vendor project.
func (ws *Workspace) ListVendorProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	return ws.listProjectFiles(projectPathJoin(vendorDir, project), project, false)
}

// IsProjectOwned returns true if the project is owned by this workspace.
func (ws *Workspace) IsProjectOwned(project ProjectPath) bool {
	ownedProjects, err := ws.OwnedProjects()
	if err != nil {
		return false
	}
	for _, p := range ownedProjects {
		if p == project {
			return true
		}
	}
	return false
}

// GetProjectLock returns the lock file for a vendor project.
func (ws *Workspace) GetProjectLock(project ProjectPath) (*LockFile, error) {
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	lockPath := lockFilePath(vendorDir, string(project))
	return readLockFile(lockPath)
}

// receiverPathJoin joins a path relative to the project receiver root.
func (r *ProjectReceiver) receiverPathJoin(relPath string) string {
	return filepath.Join(r.projectRoot, relPath)
}

// CreateFile creates a file in the project.
func (r *ProjectReceiver) CreateFile(relPath string) (*ProjectFileWriter, error) {
	absPath := r.receiverPathJoin(relPath)

	// Create directory if needed
	dir := filepath.Dir(absPath)
	if err := utils.CreateDir(dir, "file"); err != nil {
		return nil, err
	}

	// Read existing file hash if exists
	var existingHash []byte
	if data, err := os.ReadFile(absPath); err == nil {
		h := sha256.Sum256(data)
		existingHash = h[:]
	}

	// Create file
	f, err := os.Create(absPath)
	if err != nil {
		return nil, fmt.Errorf("create file: %w", err)
	}

	return &ProjectFileWriter{
		file:         f,
		hash:         sha256.New(),
		existingHash: existingHash,
		onClose: func(changed bool) {
			if changed {
				r.changed++
			}
		},
	}, nil
}

// DeleteFile deletes a file from the project.
func (r *ProjectReceiver) DeleteFile(relPath string) error {
	absPath := r.receiverPathJoin(relPath)
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	r.deleted++
	return nil
}

// Finish completes the receive operation.
func (r *ProjectReceiver) Finish() (*ReceiveStats, error) {
	// Ensure project directory exists
	if err := utils.CreateDir(r.projectRoot, "project"); err != nil {
		return nil, err
	}

	// Write lock file
	lockPath := r.receiverPathJoin(lockFileName)
	if err := writeLockFile(lockPath, &LockFile{Snapshot: string(r.snapshot)}); err != nil {
		return nil, fmt.Errorf("write lock file: %w", err)
	}

	// Write .gitattributes
	gitattrsPath := r.receiverPathJoin(gitattributesName)
	if err := os.WriteFile(gitattrsPath, []byte("* linguist-generated=true\n"), 0644); err != nil {
		return nil, fmt.Errorf("write gitattributes: %w", err)
	}

	return &ReceiveStats{
		FilesChanged: r.changed,
		FilesDeleted: r.deleted,
	}, nil
}

// readConfig reads the protato.yaml config file.
func readConfig(path string) (*Config, error) {
	return utils.ReadYAMLFile[Config](path)
}

// writeConfig writes the protato.yaml config file.
func writeConfig(path string, config *Config) error {
	return utils.WriteYAML(path, config)
}

// readLockFile reads a protato.lock file.
func readLockFile(path string) (*LockFile, error) {
	return utils.ReadYAMLFile[LockFile](path)
}

// writeLockFile writes a protato.lock file.
func writeLockFile(path string, lock *LockFile) error {
	return utils.WriteYAML(path, lock)
}

// OrphanedFiles finds files that don't belong to any known project.
// Checks both owned and vendor directories.
func (ws *Workspace) OrphanedFiles(ctx context.Context) ([]string, error) {
	var orphaned []string

	// Get owned projects
	ownedProjects, err := ws.OwnedProjects()
	if err != nil {
		return nil, err
	}
	ownedSet := ws.projectPathsToMap(ownedProjects)

	// Get received projects
	received, err := ws.ReceivedProjects(ctx)
	if err != nil {
		return nil, err
	}
	receivedSet := ws.receivedProjectsToMap(received)

	// Check vendor directory for orphaned files first
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	vendorOrphans, err := ws.findOrphanedInDir(vendorDir, receivedSet, "")
	if err != nil {
		return nil, err
	}
	orphaned = append(orphaned, vendorOrphans...)

	// Check owned directory for orphaned files
	// Exclude vendor directory from owned directory walk to avoid checking vendor files against owned projects
	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return nil, err
	}
	ownedOrphans, err := ws.findOrphanedInDir(ownedDir, ownedSet, vendorDir)
	if err != nil {
		return nil, err
	}
	orphaned = append(orphaned, ownedOrphans...)

	return orphaned, nil
}

// findOrphanedInDir finds files in a directory that don't belong to known projects.
// If excludeDir is not empty, that directory will be excluded from the walk.
func (ws *Workspace) findOrphanedInDir(dirPath string, knownProjects map[string]bool, excludeDir string) ([]string, error) {
	absDirPath, err := utils.AbsPath(dirPath)
	if err != nil {
		return nil, err
	}

	var absExcludeDir string
	if excludeDir != "" {
		absExcludeDir, err = utils.AbsPath(excludeDir)
		if err != nil {
			return nil, err
		}
	}

	if utils.DirNotExists(absDirPath) {
		return []string{}, nil
	}

	var orphaned []string
	err = filepath.WalkDir(absDirPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() {
			if excludeDir != "" && p == absExcludeDir {
				return filepath.SkipDir
			}
			return nil
		}

		if orphanPath := ws.checkIfOrphaned(p, absDirPath, d.Name(), knownProjects); orphanPath != "" {
			orphaned = append(orphaned, orphanPath)
		}
		return nil
	})

	return orphaned, err
}

// checkIfOrphaned checks if a file is orphaned and returns its repo-relative path if so.
func (ws *Workspace) checkIfOrphaned(filePath, absDirPath, fileName string, knownProjects map[string]bool) string {
	if !strings.HasSuffix(fileName, protoFileExt) {
		return ""
	}

	relPath, err := utils.RelPathToSlash(absDirPath, filePath)
	if err != nil {
		return ""
	}

	if ws.fileBelongsToProject(relPath, knownProjects) {
		return ""
	}

	repoRelPath, _ := filepath.Rel(ws.root, filePath)
	return repoRelPath
}

// projectPathsToMap converts a slice of ProjectPath to a map for fast lookups.
func (ws *Workspace) projectPathsToMap(projects []ProjectPath) map[string]bool {
	return utils.SliceToMap(projects, func(p ProjectPath) string {
		return string(p)
	})
}

// receivedProjectsToMap converts a slice of ReceivedProject to a map for fast lookups.
func (ws *Workspace) receivedProjectsToMap(projects []*ReceivedProject) map[string]bool {
	return utils.SliceToMap(projects, func(r *ReceivedProject) string {
		return string(r.Project)
	})
}

// fileBelongsToProject checks if a file belongs to any known project.
func (ws *Workspace) fileBelongsToProject(relPath string, knownProjects map[string]bool) bool {
	return utils.PathBelongsToAny(relPath, knownProjects)
}

// GetRegistryPath gets the registry path for a local project path string.
func (ws *Workspace) GetRegistryPath(projectPath string) (ProjectPath, error) {
	return ws.RegistryProjectPath(ProjectPath(projectPath))
}

// GetRegistryPathForProject gets the registry path for a local project path with error handling.
func (ws *Workspace) GetRegistryPathForProject(project ProjectPath) (ProjectPath, error) {
	registryPath, err := ws.RegistryProjectPath(project)
	if err != nil {
		return "", fmt.Errorf("get registry path for %s: %w", project, err)
	}
	return registryPath, nil
}
