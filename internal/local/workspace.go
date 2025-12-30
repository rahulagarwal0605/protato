package local

import (
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
	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/rahulagarwal0605/protato/internal/git"
)

const (
	configFileName    = "protato.yaml"
	lockFileName      = "protato.lock"
	gitattributesName = ".gitattributes"
	defaultOwnedDir   = "proto"        // Default for owned protos
	defaultVendorDir  = "vendor-proto" // Default for consumed protos
)

var (
	// ErrAlreadyInitialized is returned when trying to init an already initialized workspace.
	ErrAlreadyInitialized = errors.New("workspace already initialized")
	// ErrNotInitialized is returned when trying to open a non-initialized workspace.
	ErrNotInitialized = errors.New("workspace not initialized")
)

// Workspace represents a local protato workspace.
type Workspace struct {
	root    string          // Repository root directory
	log     *zerolog.Logger // Logger
	ignores []string        // Ignore patterns (doublestar)
	config  *Config         // Loaded configuration
}

// Init initializes a new workspace.
func Init(root string, opts InitOptions, log *zerolog.Logger) (*Workspace, error) {
	configPath := filepath.Join(root, configFileName)

	// Check if already initialized
	if _, err := os.Stat(configPath); err == nil && !opts.Force {
		return nil, ErrAlreadyInitialized
	}

	// Determine directory names (use defaults if not specified)
	ownedDir := opts.OwnedDir
	if ownedDir == "" {
		ownedDir = defaultOwnedDir
	}
	vendorDir := opts.VendorDir
	if vendorDir == "" {
		vendorDir = defaultVendorDir
	}

	// Create initial config
	config := &Config{
		Service: opts.Service,
		Directories: DirectoryConfig{
			Owned:  ownedDir,
			Vendor: vendorDir,
		},
		AutoDiscover: opts.AutoDiscover,
		Projects:     opts.Projects,
		Ignores:      []string{},
	}

	// Write config file
	if err := writeConfig(configPath, config); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Create owned protos directory
	ownedPath := filepath.Join(root, ownedDir)
	if err := os.MkdirAll(ownedPath, 0755); err != nil {
		return nil, fmt.Errorf("create owned protos dir: %w", err)
	}

	// Create vendor protos directory
	vendorPath := filepath.Join(root, vendorDir)
	if err := os.MkdirAll(vendorPath, 0755); err != nil {
		return nil, fmt.Errorf("create vendor protos dir: %w", err)
	}

	return &Workspace{
		root:    root,
		log:     log,
		ignores: config.Ignores,
		config:  config,
	}, nil
}

// Open opens an existing workspace.
func Open(root string, opts OpenOptions, log *zerolog.Logger) (*Workspace, error) {
	configPath := filepath.Join(root, configFileName)

	// Check if initialized
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		if opts.CreateIfMissing {
			return Init(root, InitOptions{}, log)
		}
		return nil, ErrNotInitialized
	}

	// Read config
	config, err := readConfig(configPath)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}

	return &Workspace{
		root:    root,
		log:     log,
		ignores: config.Ignores,
		config:  config,
	}, nil
}

// Root returns the workspace root directory.
func (ws *Workspace) Root() string {
	return ws.root
}

// OwnedDir returns the directory path for owned (producer) protos.
func (ws *Workspace) OwnedDir() string {
	if ws.config != nil && ws.config.Directories.Owned != "" {
		return filepath.Join(ws.root, ws.config.Directories.Owned)
	}
	return filepath.Join(ws.root, defaultOwnedDir)
}

// VendorDir returns the directory path for consumed (vendor) protos.
func (ws *Workspace) VendorDir() string {
	if ws.config != nil && ws.config.Directories.Vendor != "" {
		return filepath.Join(ws.root, ws.config.Directories.Vendor)
	}
	return filepath.Join(ws.root, defaultVendorDir)
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
// If AutoDiscover is enabled, it discovers projects by scanning the owned directory
// for subdirectories that contain .proto files.
func (ws *Workspace) OwnedProjects() ([]ProjectPath, error) {
	// If auto-discover is enabled, discover projects from the owned directory
	if ws.config.AutoDiscover {
		return ws.discoverProjects()
	}

	// Otherwise, use the explicit projects list
	projects := make([]ProjectPath, len(ws.config.Projects))
	for i, p := range ws.config.Projects {
		projects[i] = ProjectPath(p)
	}
	return projects, nil
}

// discoverProjects scans the owned directory and discovers projects.
// A project is any directory that contains at least one .proto file.
// Projects with a protato.lock file are excluded (they are pulled, not owned).
func (ws *Workspace) discoverProjects() ([]ProjectPath, error) {
	var projects []ProjectPath
	seen := make(map[string]bool)

	ownedPath := ws.OwnedDir()
	if _, err := os.Stat(ownedPath); os.IsNotExist(err) {
		return projects, nil
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

		// Check ignores
		if ws.shouldIgnoreProject(relDir) {
			return nil
		}

		// Skip if this is a pulled project (has protato.lock file)
		if ws.isPulledProject(relDir) {
			return nil
		}

		projects = append(projects, ProjectPath(relDir))
		return nil
	})

	return projects, err
}

// shouldIgnoreProject checks if a project path should be ignored.
func (ws *Workspace) shouldIgnoreProject(projectPath string) bool {
	for _, pattern := range ws.ignores {
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
	for _, p := range ws.config.Projects {
		owned[p] = true
		// Also add with service prefix
		if ws.config.Service != "" {
			owned[path.Join(ws.config.Service, p)] = true
		}
	}

	// Walk vendor directory looking for lock files
	err := filepath.WalkDir(vendorPath, func(p string, d fs.DirEntry, err error) error {
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
			ws.log.Warn().Err(err).Str("path", p).Msg("Failed to read lock file")
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
	// Add to existing projects
	existing := make(map[string]bool)
	for _, p := range ws.config.Projects {
		existing[p] = true
	}

	for _, p := range projects {
		ps := string(p)
		if !existing[ps] {
			ws.config.Projects = append(ws.config.Projects, ps)
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

		// Check ignores
		if ws.shouldIgnore(project, relPath) {
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

// shouldIgnore checks if a file should be ignored.
func (ws *Workspace) shouldIgnore(project ProjectPath, file string) bool {
	fullPath := path.Join(string(project), file)

	for _, pattern := range ws.ignores {
		match, _ := doublestar.Match(pattern, fullPath)
		if match {
			return true
		}
	}
	return false
}

// IsProjectOwned returns true if the project is owned by this workspace.
func (ws *Workspace) IsProjectOwned(project ProjectPath) bool {
	for _, p := range ws.config.Projects {
		if p == string(project) {
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
