package local

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"gopkg.in/yaml.v3"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

const (
	configFileName    = "protato.yaml"
	lockFileName      = "protato.lock"
	gitattributesName = ".gitattributes"
	protoFileExt      = ".proto"
)

var (
	// ErrAlreadyInitialized is returned when trying to init an already initialized workspace.
	ErrAlreadyInitialized = errors.New("workspace already initialized")
	// ErrNotInitialized is returned when trying to open a non-initialized workspace.
	ErrNotInitialized = errors.New("workspace not initialized")
)

// Workspace represents a local protato workspace.
type Workspace struct {
	root   string  // Repository root directory
	config *Config // Loaded configuration
}

// Init initializes a new workspace.
func Init(ctx context.Context, root string, config *Config, force bool) (*Workspace, error) {
	configPath := filepath.Join(root, configFileName)

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
	if err := createDirectories(root, ownedDir, vendorDir); err != nil {
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
		config.Projects = mergeStringSlice(config.Projects, new.Projects)
	}

	// Auto-discover wins - use the provided value
	config.AutoDiscover = new.AutoDiscover

	// Merge ignores if provided
	if len(new.Ignores) > 0 {
		config.Ignores = mergeStringSlice(config.Ignores, new.Ignores)
	}

	return config
}

// mergeStringSlice merges new items into existing slice, avoiding duplicates.
func mergeStringSlice(existing, newItems []string) []string {
	seen := make(map[string]bool)
	for _, item := range existing {
		seen[item] = true
	}

	result := existing
	for _, item := range newItems {
		if !seen[item] {
			result = append(result, item)
			seen[item] = true
		}
	}
	return result
}

// createDirectories creates owned and vendor directories.
func createDirectories(root, ownedDir, vendorDir string) error {
	ownedPath := filepath.Join(root, ownedDir)
	if err := os.MkdirAll(ownedPath, 0755); err != nil {
		return fmt.Errorf("create owned protos dir: %w", err)
	}

	vendorPath := filepath.Join(root, vendorDir)
	if err := os.MkdirAll(vendorPath, 0755); err != nil {
		return fmt.Errorf("create vendor protos dir: %w", err)
	}

	return nil
}

// Open opens an existing workspace.
func Open(ctx context.Context, root string) (*Workspace, error) {
	configPath := filepath.Join(root, configFileName)

	// Check if initialized
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
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

// OwnedDir returns the absolute directory path for owned (producer) protos.
func (ws *Workspace) OwnedDir() (string, error) {
	dir, err := ws.config.OwnedDir()
	if err != nil {
		return "", fmt.Errorf("get owned directory: %w", err)
	}
	return filepath.Join(ws.root, dir), nil
}

// OwnedDirName returns just the owned directory name (e.g., "proto") without the root path.
func (ws *Workspace) OwnedDirName() (string, error) {
	return ws.config.OwnedDir()
}

// VendorDir returns the directory path for consumed (vendor) protos.
func (ws *Workspace) VendorDir() (string, error) {
	dir, err := ws.config.VendorDir()
	if err != nil {
		return "", fmt.Errorf("get vendor directory: %w", err)
	}
	return filepath.Join(ws.root, dir), nil
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
	return ProjectPath(path.Join(ws.config.Service, string(localProject))), nil
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

	var projects []ProjectPath
	seen := make(map[string]bool)

	if _, err := os.Stat(ownedPath); os.IsNotExist(err) {
		return []ProjectPath{}, nil
	}

	// Walk the owned directory to find all .proto files
	err = filepath.WalkDir(ownedPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip non-proto files
		if d.IsDir() || !strings.HasSuffix(d.Name(), protoFileExt) {
			return nil
		}

		// Get the directory containing this proto file
		protoDir := filepath.Dir(p)

		// Calculate path relative to owned directory (for pattern matching and return value)
		relToOwned, err := filepath.Rel(ownedPath, protoDir)
		if err != nil {
			return nil
		}
		relToOwned = filepath.ToSlash(relToOwned)

		// Apply filter pattern if provided (match against owned-directory-relative path)
		if filterPattern != nil {
			if !ws.matchesPattern(relToOwned, []string{*filterPattern}) {
				return nil
			}
		}

		// Skip if we've already seen this project
		if seen[relToOwned] {
			return nil
		}
		seen[relToOwned] = true

		// Filter out pulled projects
		isPulled, err := ws.isPulledProject(relToOwned)
		if err != nil {
			return err
		}
		if isPulled {
			return nil
		}

		projects = append(projects, ProjectPath(relToOwned))
		return nil
	})

	return projects, err
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
	seen := make(map[string]bool)
	var deduplicated []ProjectPath
	for _, p := range allMatches {
		if !seen[string(p)] {
			deduplicated = append(deduplicated, p)
			seen[string(p)] = true
		}
	}

	return deduplicated, nil
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
		match, _ := doublestar.Match(pattern, projectPath)
		if match {
			return true
		}
		// Also check with trailing slash to match directory patterns
		match, _ = doublestar.Match(pattern, projectPath+"/")
		if match {
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
	lockPath := filepath.Join(ownedDir, projectPath, lockFileName)
	if _, err := os.Stat(lockPath); err == nil {
		return true, nil
	}

	// Also check in vendor directory if different
	if ownedDir != vendorDir {
		lockPath = filepath.Join(vendorDir, projectPath, lockFileName)
		if _, err := os.Stat(lockPath); err == nil {
			return true, nil
		}
	}

	return false, nil
}

// ReceivedProjects returns the list of received (pulled) projects.
func (ws *Workspace) ReceivedProjects(ctx context.Context) ([]*ReceivedProject, error) {
	var received []*ReceivedProject

	// Look in vendor directory for received projects
	vendorPath, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	if _, err := os.Stat(vendorPath); os.IsNotExist(err) {
		return received, nil
	}

	// Get owned projects for filtering
	owned := make(map[string]bool)
	ownedProjects, err := ws.OwnedProjects()
	if err != nil {
		return nil, err
	}
	for _, p := range ownedProjects {
		owned[string(p)] = true
		// Also add with service prefix
		if ws.config.Service != "" {
			owned[path.Join(ws.config.Service, string(p))] = true
		}
	}

	// Walk vendor directory looking for lock files
	err = filepath.WalkDir(vendorPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != lockFileName {
			return nil
		}

		// Get project path from lock file location
		relPath, err := filepath.Rel(vendorPath, filepath.Dir(p))
		if err != nil {
			return nil
		}
		projectPath := filepath.ToSlash(relPath)

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
	existing := make(map[string]bool)
	for _, p := range ws.config.Projects {
		existing[p] = true
	}

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
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			return fmt.Errorf("create project dir: %w", err)
		}
	}

	// Write updated config
	configPath := filepath.Join(ws.root, configFileName)
	return writeConfig(configPath, ws.config)
}

// ReceiveProject starts receiving a project (into vendor directory).
func (ws *Workspace) ReceiveProject(req *ReceiveProjectRequest) (*ProjectReceiver, error) {
	// Received projects go into the vendor directory
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	projectRoot := filepath.Join(vendorDir, string(req.Project))
	return &ProjectReceiver{
		ws:          ws,
		project:     req.Project,
		projectRoot: projectRoot,
		snapshot:    req.Snapshot,
	}, nil
}

// ListOwnedProjectFiles lists all files in an owned project.
// project: path relative to the owned directory (e.g., "api/v1")
func (ws *Workspace) ListOwnedProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	var files []ProjectFile

	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return files, err
	}
	projectPath := filepath.Join(ownedDir, string(project))
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return files, nil
	}

	err = filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		// Only process .proto files (skip directories and non-proto files)
		if d.IsDir() || !strings.HasSuffix(d.Name(), protoFileExt) {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(projectPath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		files = append(files, ProjectFile{
			Path:         relPath,
			AbsolutePath: p,
		})

		return nil
	})
	if err != nil {
		return files, err
	}

	// Apply ignores: filter out files matching ignore patterns
	files = ws.applyFileIgnores(files, project)

	return files, nil
}

// ListVendorProjectFiles lists all files in a vendor project.
func (ws *Workspace) ListVendorProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	var files []ProjectFile

	vendorDir, err := ws.VendorDir()
	if err != nil {
		return files, err
	}
	projectPath := filepath.Join(vendorDir, string(project))
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return files, nil
	}

	err = filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Skip special files
		name := d.Name()
		if name == lockFileName || name == gitattributesName {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(projectPath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		files = append(files, ProjectFile{
			Path:         relPath,
			AbsolutePath: p,
		})

		return nil
	})

	return files, err
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
	lockPath := filepath.Join(vendorDir, string(project), lockFileName)
	return readLockFile(lockPath)
}

// ProjectReceiver handles receiving files for a project.
type ProjectReceiver struct {
	ws          *Workspace
	project     ProjectPath
	projectRoot string
	snapshot    git.Hash
	changed     int
	deleted     int
}

// CreateFile creates a file in the project.
func (r *ProjectReceiver) CreateFile(relPath string) (*ProjectFileWriter, error) {
	absPath := filepath.Join(r.projectRoot, relPath)

	// Create directory if needed
	dir := filepath.Dir(absPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
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
	absPath := filepath.Join(r.projectRoot, relPath)
	if err := os.Remove(absPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	r.deleted++
	return nil
}

// Finish completes the receive operation.
func (r *ProjectReceiver) Finish() (*ReceiveStats, error) {
	// Write lock file
	lockPath := filepath.Join(r.projectRoot, lockFileName)
	if err := writeLockFile(lockPath, &LockFile{Snapshot: string(r.snapshot)}); err != nil {
		return nil, fmt.Errorf("write lock file: %w", err)
	}

	// Write .gitattributes
	gitattrsPath := filepath.Join(r.projectRoot, gitattributesName)
	if err := os.WriteFile(gitattrsPath, []byte("* linguist-generated=true\n"), 0644); err != nil {
		return nil, fmt.Errorf("write gitattributes: %w", err)
	}

	return &ReceiveStats{
		FilesChanged: r.changed,
		FilesDeleted: r.deleted,
	}, nil
}

// ProjectFileWriter handles writing a project file.
type ProjectFileWriter struct {
	file         *os.File
	hash         hash.Hash
	existingHash []byte
	onClose      func(changed bool)
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
	changed := len(w.existingHash) == 0 || !hashEqual(newHash, w.existingHash)
	w.onClose(changed)

	return err
}

// hashEqual compares two hash slices.
func hashEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// readConfig reads the protato.yaml config file.
func readConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return &config, nil
}

// writeConfig writes the protato.yaml config file.
func writeConfig(path string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// readLockFile reads a protato.lock file.
func readLockFile(path string) (*LockFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var lock LockFile
	if err := yaml.Unmarshal(data, &lock); err != nil {
		return nil, err
	}

	return &lock, nil
}

// writeLockFile writes a protato.lock file.
func writeLockFile(path string, lock *LockFile) error {
	data, err := yaml.Marshal(lock)
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// ValidateProjectPath validates a project path.
func ValidateProjectPath(p string) error {
	if p == "" {
		return errors.New("project path cannot be empty")
	}
	if strings.Contains(p, "\\") {
		return errors.New("project path cannot contain backslashes")
	}
	if strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return errors.New("project path cannot have leading or trailing slashes")
	}
	if !fs.ValidPath(p) {
		return errors.New("invalid project path")
	}
	return nil
}

// ProjectsOverlap checks if any two project paths overlap.
func ProjectsOverlap(projects []string) error {
	for i, p1 := range projects {
		for j, p2 := range projects {
			if i == j {
				continue
			}
			if strings.HasPrefix(p1+"/", p2+"/") || strings.HasPrefix(p2+"/", p1+"/") {
				return fmt.Errorf("projects overlap: %s and %s", p1, p2)
			}
		}
	}
	return nil
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
	ownedSet := make(map[string]bool)
	for _, p := range ownedProjects {
		ownedSet[string(p)] = true
	}

	// Get received projects
	received, err := ws.ReceivedProjects(ctx)
	if err != nil {
		return nil, err
	}
	receivedSet := make(map[string]bool)
	for _, r := range received {
		receivedSet[string(r.Project)] = true
	}

	// Check owned directory for orphaned files
	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return nil, err
	}
	ownedOrphans, err := ws.findOrphanedInDir(ownedDir, ownedSet)
	if err != nil {
		return nil, err
	}
	orphaned = append(orphaned, ownedOrphans...)

	// Check vendor directory for orphaned files
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return nil, err
	}
	vendorOrphans, err := ws.findOrphanedInDir(vendorDir, receivedSet)
	if err != nil {
		return nil, err
	}
	orphaned = append(orphaned, vendorOrphans...)

	return orphaned, nil
}

// findOrphanedInDir finds files in a directory that don't belong to known projects.
func (ws *Workspace) findOrphanedInDir(dirPath string, knownProjects map[string]bool) ([]string, error) {
	var orphaned []string

	if _, err := os.Stat(dirPath); os.IsNotExist(err) {
		return orphaned, nil
	}

	err := filepath.WalkDir(dirPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Skip special files
		name := d.Name()
		if name == lockFileName || name == gitattributesName {
			return nil
		}

		// Get relative path from directory
		relPath, err := filepath.Rel(dirPath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check if file belongs to any known project
		belongsToProject := false
		for proj := range knownProjects {
			if strings.HasPrefix(relPath, proj+"/") || relPath == proj {
				belongsToProject = true
				break
			}
		}

		if !belongsToProject {
			// Return path relative to repo root
			repoRelPath, _ := filepath.Rel(ws.root, p)
			orphaned = append(orphaned, repoRelPath)
		}

		return nil
	})

	return orphaned, err
}
