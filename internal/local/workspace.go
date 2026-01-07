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
)

var (
	// ErrAlreadyInitialized is returned when trying to init an already initialized workspace.
	ErrAlreadyInitialized = errors.New("workspace already initialized")
	// ErrNotInitialized is returned when trying to open a non-initialized workspace.
	ErrNotInitialized = errors.New("workspace not initialized")
)

// Workspace represents a local protato workspace.
type Workspace struct {
	root   string          // Repository root directory
	ctx    context.Context // Context for dependency injection (logger)
	config *Config         // Loaded configuration
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
	ownedDir := finalConfig.OwnedDir()
	vendorDir := finalConfig.VendorDir()
	if err := createDirectories(root, ownedDir, vendorDir); err != nil {
		return nil, err
	}

	return &Workspace{
		root:   root,
		ctx:    ctx,
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

	// Merge includes if provided
	if len(new.Includes) > 0 {
		config.Includes = mergeStringSlice(config.Includes, new.Includes)
	}

	// Auto-discover wins - use the provided value
	config.AutoDiscover = new.AutoDiscover

	// Merge excludes if provided
	if len(new.Excludes) > 0 {
		config.Excludes = mergeStringSlice(config.Excludes, new.Excludes)
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
func Open(ctx context.Context, root string, opts OpenOptions) (*Workspace, error) {
	configPath := filepath.Join(root, configFileName)

	// Check if initialized
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if opts.CreateIfMissing {
			defaultConfig := &Config{
				Directories: DefaultDirectoryConfig(),
			}
			return Init(ctx, root, defaultConfig, false)
		}
		return nil, ErrNotInitialized
	}

	// Read config
	config, err := readConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return &Workspace{
		root:   root,
		ctx:    ctx,
		config: config,
	}, nil
}

// Root returns the workspace root directory.
func (ws *Workspace) Root() string {
	return ws.root
}

// OwnedDir returns the directory path for owned (producer) protos.
func (ws *Workspace) OwnedDir() string {
	return filepath.Join(ws.root, ws.config.OwnedDir())
}

// VendorDir returns the directory path for consumed (vendor) protos.
func (ws *Workspace) VendorDir() string {
	return filepath.Join(ws.root, ws.config.VendorDir())
}

// ServiceName returns the service name for registry namespacing.
func (ws *Workspace) ServiceName() string {
	if ws.config != nil {
		return ws.config.Service
	}
	return ""
}

// RegistryProjectPath returns the full registry path for a local project.
// It prefixes the project path with the service name if configured.
func (ws *Workspace) RegistryProjectPath(localProject ProjectPath) ProjectPath {
	if ws.config != nil && ws.config.Service != "" {
		return ProjectPath(path.Join(ws.config.Service, string(localProject)))
	}
	return localProject
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
// When auto_discover=true: includes all projects in owned dir + projects matching includes patterns, minus excludes
// When auto_discover=false: includes only projects matching includes patterns, minus excludes
func (ws *Workspace) OwnedProjects() ([]ProjectPath, error) {
	if ws.config.AutoDiscover {
		return ws.discoverProjects()
	}

	// When auto-discover is false, only include projects matching includes patterns
	return ws.getProjectsFromIncludes()
}

// discoverProjects scans the owned directory and discovers projects.
// When auto_discover=true: includes all projects in owned dir + projects matching includes patterns, minus excludes
func (ws *Workspace) discoverProjects() ([]ProjectPath, error) {
	var allProjects []ProjectPath
	seen := make(map[string]bool)

	ownedPath := ws.OwnedDir()
	if _, err := os.Stat(ownedPath); os.IsNotExist(err) {
		return ws.applyIncludesAndExcludes([]ProjectPath{}, allProjects)
	}

	// Walk the owned directory to find all .proto files
	err := filepath.WalkDir(ownedPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip non-proto files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}

		// Get the directory containing this proto file
		protoDir := filepath.Dir(p)
		relDir, err := filepath.Rel(ownedPath, protoDir)
		if err != nil {
			return nil
		}
		relDir = filepath.ToSlash(relDir)

		// Skip if we've already seen this project
		if seen[relDir] {
			return nil
		}
		seen[relDir] = true

		// Skip if this is a pulled project (has protato.lock file)
		if ws.isPulledProject(relDir) {
			return nil
		}

		allProjects = append(allProjects, ProjectPath(relDir))
		return nil
	})

	if err != nil {
		return nil, err
	}

	return ws.applyIncludesAndExcludes(allProjects, []ProjectPath{})
}

// getProjectsFromIncludes returns projects matching includes patterns when auto-discover is false.
func (ws *Workspace) getProjectsFromIncludes() ([]ProjectPath, error) {
	if len(ws.config.Includes) == 0 {
		return []ProjectPath{}, nil
	}

	var matchedProjects []ProjectPath
	seen := make(map[string]bool)

	ownedPath := ws.OwnedDir()
	if _, err := os.Stat(ownedPath); os.IsNotExist(err) {
		return []ProjectPath{}, nil
	}

	// Walk the owned directory to find projects matching includes patterns
	err := filepath.WalkDir(ownedPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip non-proto files
		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}

		// Get the directory containing this proto file
		protoDir := filepath.Dir(p)
		relDir, err := filepath.Rel(ownedPath, protoDir)
		if err != nil {
			return nil
		}
		relDir = filepath.ToSlash(relDir)

		// Skip if we've already seen this project
		if seen[relDir] {
			return nil
		}
		seen[relDir] = true

		// Skip if this is a pulled project (has protato.lock file)
		if ws.isPulledProject(relDir) {
			return nil
		}

		// Check if project matches any include pattern
		if ws.matchesIncludes(relDir) {
			matchedProjects = append(matchedProjects, ProjectPath(relDir))
		}

		return nil
	})

	if err != nil {
		return nil, err
	}

	return ws.applyIncludesAndExcludes([]ProjectPath{}, matchedProjects)
}

// applyIncludesAndExcludes applies include and exclude patterns to projects.
// discoveredProjects: projects found by scanning (when auto-discover=true)
// includedProjects: projects matching includes patterns (when auto-discover=false or additional when true)
func (ws *Workspace) applyIncludesAndExcludes(discoveredProjects []ProjectPath, includedProjects []ProjectPath) ([]ProjectPath, error) {
	result := make(map[string]ProjectPath)

	// Add discovered projects (when auto-discover=true, these are all projects in owned dir)
	for _, p := range discoveredProjects {
		result[string(p)] = p
	}

	// Add projects matching includes patterns
	// When auto-discover=true: these are additional to discovered projects
	// When auto-discover=false: these are the only projects
	if len(ws.config.Includes) > 0 {
		ownedPath := ws.OwnedDir()
		for _, includePattern := range ws.config.Includes {
			// Find all projects matching this include pattern
			matches, err := ws.findProjectsMatchingPattern(ownedPath, includePattern)
			if err != nil {
				return nil, err
			}
			for _, p := range matches {
				if !ws.isPulledProject(string(p)) {
					result[string(p)] = p
				}
			}
		}
	}

	// Add explicitly included projects from includedProjects parameter
	for _, p := range includedProjects {
		result[string(p)] = p
	}

	// Apply excludes
	finalProjects := []ProjectPath{}
	for _, p := range result {
		if !ws.shouldExcludeProject(string(p)) {
			finalProjects = append(finalProjects, p)
		}
	}

	return finalProjects, nil
}

// findProjectsMatchingPattern finds all projects matching a glob pattern.
func (ws *Workspace) findProjectsMatchingPattern(ownedPath, pattern string) ([]ProjectPath, error) {
	var matches []ProjectPath
	seen := make(map[string]bool)

	err := filepath.WalkDir(ownedPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		if d.IsDir() || !strings.HasSuffix(d.Name(), ".proto") {
			return nil
		}

		protoDir := filepath.Dir(p)
		relDir, err := filepath.Rel(ownedPath, protoDir)
		if err != nil {
			return nil
		}
		relDir = filepath.ToSlash(relDir)

		if seen[relDir] {
			return nil
		}
		seen[relDir] = true

		// Check if matches pattern
		match, _ := doublestar.Match(pattern, relDir)
		if match {
			matches = append(matches, ProjectPath(relDir))
		}

		return nil
	})

	return matches, err
}

// matchesIncludes checks if a project path matches any include pattern.
func (ws *Workspace) matchesIncludes(projectPath string) bool {
	if len(ws.config.Includes) == 0 {
		return false
	}

	for _, pattern := range ws.config.Includes {
		match, _ := doublestar.Match(pattern, projectPath)
		if match {
			return true
		}
		// Also check with trailing slash
		match, _ = doublestar.Match(pattern, projectPath+"/")
		if match {
			return true
		}
	}
	return false
}

// shouldExcludeProject checks if a project path should be excluded.
func (ws *Workspace) shouldExcludeProject(projectPath string) bool {
	if ws.config == nil {
		return false
	}

	for _, pattern := range ws.config.Excludes {
		// Check if the pattern matches the project path
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
func (ws *Workspace) isPulledProject(projectPath string) bool {
	// Check in owned directory (for when owned == vendor)
	lockPath := filepath.Join(ws.OwnedDir(), projectPath, lockFileName)
	if _, err := os.Stat(lockPath); err == nil {
		return true
	}

	// Also check in vendor directory if different
	if ws.OwnedDir() != ws.VendorDir() {
		lockPath = filepath.Join(ws.VendorDir(), projectPath, lockFileName)
		if _, err := os.Stat(lockPath); err == nil {
			return true
		}
	}

	return false
}

// ReceivedProjects returns the list of received (pulled) projects.
func (ws *Workspace) ReceivedProjects() ([]*ReceivedProject, error) {
	var received []*ReceivedProject

	// Look in vendor directory for received projects
	vendorPath := ws.VendorDir()
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
			logger.Log(ws.ctx).Warn().Err(err).Str("path", p).Msg("Failed to read lock file")
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
func (ws *Workspace) AddOwnedProjects(projects []ProjectPath) error {
	// Add to existing includes
	existing := make(map[string]bool)
	for _, p := range ws.config.Includes {
		existing[p] = true
	}

	for _, p := range projects {
		ps := string(p)
		if !existing[ps] {
			ws.config.Includes = append(ws.config.Includes, ps)
			existing[ps] = true
		}

		// Create project directory in owned directory
		projectPath := filepath.Join(ws.OwnedDir(), ps)
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			return fmt.Errorf("create project dir: %w", err)
		}
	}

	// Write updated config
	configPath := filepath.Join(ws.root, configFileName)
	return writeConfig(configPath, ws.config)
}

// ReceiveProject starts receiving a project (into vendor directory).
func (ws *Workspace) ReceiveProject(req *ReceiveProjectRequest) *ProjectReceiver {
	// Received projects go into the vendor directory
	projectRoot := filepath.Join(ws.VendorDir(), string(req.Project))
	return &ProjectReceiver{
		ws:          ws,
		project:     req.Project,
		projectRoot: projectRoot,
		snapshot:    req.Snapshot,
	}
}

// ListOwnedProjectFiles lists all files in an owned project.
func (ws *Workspace) ListOwnedProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	var files []ProjectFile

	projectPath := filepath.Join(ws.OwnedDir(), string(project))
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return files, nil
	}

	err := filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Skip special files
		name := d.Name()
		if name == lockFileName || name == gitattributesName || name == "protato.root.yaml" {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(projectPath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check excludes
		if ws.shouldExclude(project, relPath) {
			return nil
		}

		files = append(files, ProjectFile{
			Path:         relPath,
			AbsolutePath: p,
		})

		return nil
	})

	return files, err
}

// ListVendorProjectFiles lists all files in a vendor project.
func (ws *Workspace) ListVendorProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	var files []ProjectFile

	projectPath := filepath.Join(ws.VendorDir(), string(project))
	if _, err := os.Stat(projectPath); os.IsNotExist(err) {
		return files, nil
	}

	err := filepath.WalkDir(projectPath, func(p string, d fs.DirEntry, err error) error {
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

// shouldExclude checks if a file should be excluded.
func (ws *Workspace) shouldExclude(project ProjectPath, file string) bool {
	if ws.config == nil {
		return false
	}

	fullPath := path.Join(string(project), file)

	for _, pattern := range ws.config.Excludes {
		match, _ := doublestar.Match(pattern, fullPath)
		if match {
			return true
		}
	}
	return false
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
	lockPath := filepath.Join(ws.VendorDir(), string(project), lockFileName)
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
func (ws *Workspace) OrphanedFiles() ([]string, error) {
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
	received, err := ws.ReceivedProjects()
	if err != nil {
		return nil, err
	}
	receivedSet := make(map[string]bool)
	for _, r := range received {
		receivedSet[string(r.Project)] = true
	}

	// Check owned directory for orphaned files
	ownedOrphans, err := ws.findOrphanedInDir(ws.OwnedDir(), ownedSet)
	if err != nil {
		return nil, err
	}
	orphaned = append(orphaned, ownedOrphans...)

	// Check vendor directory for orphaned files
	vendorOrphans, err := ws.findOrphanedInDir(ws.VendorDir(), receivedSet)
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
