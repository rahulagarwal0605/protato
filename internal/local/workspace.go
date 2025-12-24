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
	protosDir         = "protos"
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

	// Create initial config
	config := &Config{
		Projects: opts.Projects,
		Ignores:  []string{},
	}

	// Write config file
	if err := writeConfig(configPath, config); err != nil {
		return nil, fmt.Errorf("write config: %w", err)
	}

	// Create protos directory
	protosPath := filepath.Join(root, protosDir)
	if err := os.MkdirAll(protosPath, 0755); err != nil {
		return nil, fmt.Errorf("create protos dir: %w", err)
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

// ProtosDir returns the protos directory path.
func (ws *Workspace) ProtosDir() string {
	return filepath.Join(ws.root, protosDir)
}

// OwnedProjects returns the list of owned projects.
func (ws *Workspace) OwnedProjects() ([]ProjectPath, error) {
	projects := make([]ProjectPath, len(ws.config.Projects))
	for i, p := range ws.config.Projects {
		projects[i] = ProjectPath(p)
	}
	return projects, nil
}

// ReceivedProjects returns the list of received (pulled) projects.
func (ws *Workspace) ReceivedProjects() ([]*ReceivedProject, error) {
	var received []*ReceivedProject

	protosPath := ws.ProtosDir()
	if _, err := os.Stat(protosPath); os.IsNotExist(err) {
		return received, nil
	}

	// Get owned projects for filtering
	owned := make(map[string]bool)
	for _, p := range ws.config.Projects {
		owned[p] = true
	}

	// Walk protos directory looking for lock files
	err := filepath.WalkDir(protosPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != lockFileName {
			return nil
		}

		// Get project path from lock file location
		relPath, err := filepath.Rel(protosPath, filepath.Dir(p))
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

// ListProjectFiles lists all files in a project.
func (ws *Workspace) ListProjectFiles(project ProjectPath) ([]ProjectFile, error) {
	var files []ProjectFile

	projectPath := filepath.Join(ws.ProtosDir(), string(project))
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

		// Create project directory
		projectPath := filepath.Join(ws.ProtosDir(), ps)
		if err := os.MkdirAll(projectPath, 0755); err != nil {
			return fmt.Errorf("create project dir: %w", err)
		}
	}

	// Write updated config
	configPath := filepath.Join(ws.root, configFileName)
	return writeConfig(configPath, ws.config)
}

// ReceiveProject starts receiving a project.
func (ws *Workspace) ReceiveProject(req *ReceiveProjectRequest) *ProjectReceiver {
	projectRoot := filepath.Join(ws.ProtosDir(), string(req.Project))
	return &ProjectReceiver{
		ws:          ws,
		project:     req.Project,
		projectRoot: projectRoot,
		snapshot:    req.Snapshot,
	}
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

// GetProjectLock returns the lock file for a project.
func (ws *Workspace) GetProjectLock(project ProjectPath) (*LockFile, error) {
	lockPath := filepath.Join(ws.ProtosDir(), string(project), lockFileName)
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

// OrphanedFiles finds files in protos/ that don't belong to any project.
func (ws *Workspace) OrphanedFiles() ([]string, error) {
	var orphaned []string

	protosPath := ws.ProtosDir()
	if _, err := os.Stat(protosPath); os.IsNotExist(err) {
		return orphaned, nil
	}

	// Get all known projects
	known := make(map[string]bool)
	for _, p := range ws.config.Projects {
		known[p] = true
	}

	received, err := ws.ReceivedProjects()
	if err != nil {
		return nil, err
	}
	for _, r := range received {
		known[string(r.Project)] = true
	}

	// Walk protos directory
	err = filepath.WalkDir(protosPath, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}

		// Get relative path
		relPath, err := filepath.Rel(protosPath, p)
		if err != nil {
			return nil
		}
		relPath = filepath.ToSlash(relPath)

		// Check if file belongs to any known project
		belongsToProject := false
		for proj := range known {
			if strings.HasPrefix(relPath, proj+"/") {
				belongsToProject = true
				break
			}
		}

		if !belongsToProject {
			orphaned = append(orphaned, filepath.Join(protosDir, relPath))
		}

		return nil
	})

	return orphaned, err
}
