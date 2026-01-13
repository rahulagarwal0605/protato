package registry

import (
	"bytes"
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"
	"syscall"

	"gopkg.in/yaml.v3"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

const (
	projectMetaFile = "protato.root.yaml"
	protosDir       = "protos"
	protoFileExt    = ".proto"
)

var (
	// ErrNotFound is returned when a project is not found.
	ErrNotFound = errors.New("project not found")
)

// gitRepository is the interface for Git operations.
type gitRepository interface {
	Fetch(context.Context, git.FetchOptions) error
	Push(context.Context, git.PushOptions) error
	RevHash(context.Context, string) (git.Hash, error)
	RevExists(context.Context, string) bool
	ReadTree(context.Context, git.Treeish, git.ReadTreeOptions) ([]git.TreeEntry, error)
	WriteObject(context.Context, io.Reader, git.WriteObjectOptions) (git.Hash, error)
	ReadObject(context.Context, git.ObjectType, git.Hash, io.Writer) error
	UpdateTree(context.Context, git.UpdateTreeRequest) (git.Hash, error)
	CommitTree(context.Context, git.CommitTreeRequest) (git.Hash, error)
	UpdateRef(context.Context, string, git.Hash, git.Hash) error
}

// Cache manages the local cache of the remote registry.
type Cache struct {
	root     string        // Cache directory path
	repo     gitRepository // Bare Git repository
	url      string        // Registry URL
	mu       sync.Mutex    // Protects concurrent access to git operations
	lockFile *os.File      // File lock for cross-process synchronization
}

// Open opens or initializes the registry cache.
func Open(ctx context.Context, cacheDir string, registryURL string) (*Cache, error) {
	// Create cache directory hash from URL
	urlHash := sha256.Sum256([]byte(registryURL))
	cacheRoot := filepath.Join(cacheDir, fmt.Sprintf("%x", urlHash[:8]))

	var repo *git.Repository
	var err error

	// Check if cache exists
	if _, statErr := os.Stat(cacheRoot); os.IsNotExist(statErr) {
		// Clone the repository
		logger.Log(ctx).Info().Msg("Cloning registry")
		repo, err = git.Clone(ctx, registryURL, cacheRoot, git.CloneOptions{
			Bare:   true,
			NoTags: true,
			Depth:  1,
		})
		if err != nil {
			return nil, fmt.Errorf("clone registry: %w", err)
		}
	} else {
		// Open existing cache
		repo, err = git.Open(ctx, cacheRoot, git.OpenOptions{Bare: true})
		if err != nil {
			return nil, fmt.Errorf("open registry cache: %w", err)
		}
	}

	cache := &Cache{
		root: cacheRoot,
		repo: repo,
		url:  registryURL,
	}

	// Acquire file lock to prevent concurrent access from multiple processes
	lockPath := filepath.Join(cacheRoot, ".protato.lock")
	lockFile, err := os.OpenFile(lockPath, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("create lock file: %w", err)
	}

	// Try to acquire exclusive lock (non-blocking)
	err = syscall.Flock(int(lockFile.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if err != nil {
		lockFile.Close()
		return nil, fmt.Errorf("cache is locked by another protato process (try: pkill protato or killall protato)")
	}

	cache.lockFile = lockFile
	logger.Log(ctx).Debug().Str("lock", lockPath).Msg("Acquired cache lock")

	return cache, nil
}

// Close releases the cache lock and closes resources.
// The lock is automatically released when the process exits, but this allows explicit cleanup.
func (r *Cache) Close() error {
	if r.lockFile != nil {
		syscall.Flock(int(r.lockFile.Fd()), syscall.LOCK_UN)
		return r.lockFile.Close()
	}
	return nil
}

// Refresh refreshes the cache from remote.
func (r *Cache) Refresh(ctx context.Context) error {
	logger.Log(ctx).Debug().Msg("Refreshing registry cache")
	branch := r.getDefaultBranch(ctx)
	return r.repo.Fetch(ctx, git.FetchOptions{
		Remote: "origin",
		RefSpecs: []git.Refspec{
			git.Refspec(fmt.Sprintf("refs/heads/%s:refs/remotes/origin/%s", branch, branch)),
		},
		Depth: 1,
		Prune: true,
		Force: true, // Force update to handle non-fast-forward (cache can be reset)
	})
}

// Snapshot returns the current registry state (Git commit hash).
func (r *Cache) Snapshot(ctx context.Context) (git.Hash, error) {
	// Try FETCH_HEAD first (for bare repos after fetch)
	hash, err := r.repo.RevHash(ctx, "FETCH_HEAD")
	if err == nil {
		return hash, nil
	}

	// Fall back to HEAD
	return r.repo.RevHash(ctx, "HEAD")
}

// LookupProject finds a project by path.
func (r *Cache) LookupProject(ctx context.Context, req *LookupProjectRequest) (*LookupProjectResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot, err := r.getOrCreateSnapshot(ctx, req.Snapshot)
	if err != nil {
		return nil, err
	}

	if !r.repo.RevExists(ctx, string(snapshot)) {
		return nil, fmt.Errorf("snapshot not found: %s", snapshot)
	}

	return r.findProjectByPath(ctx, snapshot, req.Path)
}

// findProjectByPath searches for a project by walking up the path hierarchy.
func (r *Cache) findProjectByPath(ctx context.Context, snapshot git.Hash, projectPath string) (*LookupProjectResponse, error) {
	for {
		response := r.tryFindProjectAtPath(ctx, snapshot, projectPath)
		if response != nil {
			return response, nil
		}

		parent := path.Dir(projectPath)
		if parent == "." || parent == projectPath {
			break
		}
		projectPath = parent
	}

	return nil, ErrNotFound
}

// tryFindProjectAtPath attempts to find a project at the given path.
func (r *Cache) tryFindProjectAtPath(ctx context.Context, snapshot git.Hash, projectPath string) *LookupProjectResponse {
	metaPath := path.Join(protosDir, projectPath, projectMetaFile)
	entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
		Paths: []string{metaPath},
	})
	if err != nil || len(entries) == 0 {
		return nil
	}

	project, err := r.readProjectMeta(ctx, entries[0].Hash)
	if err != nil {
		return nil
	}
	project.Path = ProjectPath(projectPath)

	projectHash := r.getProjectTreeHash(ctx, snapshot, projectPath)
	return &LookupProjectResponse{
		Project:     project,
		Snapshot:    snapshot,
		ProjectHash: projectHash,
	}
}

// getProjectTreeHash retrieves the tree hash for a project path.
func (r *Cache) getProjectTreeHash(ctx context.Context, snapshot git.Hash, projectPath string) git.Hash {
	projTreePath := path.Join(protosDir, projectPath)
	treeEntries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
		Paths: []string{projTreePath},
	})
	if err == nil && len(treeEntries) > 0 {
		return treeEntries[0].Hash
	}
	return git.Hash("")
}

// readProjectMeta reads a project metadata file.
func (r *Cache) readProjectMeta(ctx context.Context, hash git.Hash) (*Project, error) {
	var buf bytes.Buffer
	if err := r.repo.ReadObject(ctx, git.BlobType, hash, &buf); err != nil {
		return nil, fmt.Errorf("read project meta: %w", err)
	}

	var meta ProjectMeta
	if err := yaml.Unmarshal(buf.Bytes(), &meta); err != nil {
		return nil, fmt.Errorf("parse project meta: %w", err)
	}

	return &Project{
		Commit:        git.Hash(meta.Git.Commit),
		RepositoryURL: meta.Git.URL,
	}, nil
}

// ListProjects lists all projects in the registry.
func (r *Cache) ListProjects(ctx context.Context, opts *ListProjectsOptions) ([]ProjectPath, error) {
	snapshot := git.Hash("")
	if opts != nil {
		snapshot = opts.Snapshot
	}
	snapshot, err := r.getOrCreateSnapshot(ctx, snapshot)
	if err != nil {
		return nil, err
	}

	// Determine search path: use prefix if provided, otherwise scan entire protos/
	searchPath := protosDir
	if opts != nil && opts.Prefix != "" {
		searchPath = path.Join(protosDir, opts.Prefix)
	}

	// List all files in search path
	entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
		Recurse: true,
		Paths:   []string{searchPath},
	})
	if err != nil {
		return nil, fmt.Errorf("read tree: %w", err)
	}

	// Find all project root files
	projectSet := make(map[string]bool)
	for _, entry := range entries {
		if entry.Type != git.BlobType {
			continue
		}
		if path.Base(entry.Path) != projectMetaFile {
			continue
		}

		// Extract project path
		dir := path.Dir(entry.Path)
		projectPath := strings.TrimPrefix(dir, protosDir+"/")

		projectSet[projectPath] = true
	}

	// Convert to slice
	var projects []ProjectPath
	for p := range projectSet {
		projects = append(projects, ProjectPath(p))
	}

	return projects, nil
}

// ListProjectFiles lists all files in a project.
func (r *Cache) ListProjectFiles(ctx context.Context, req *ListProjectFilesRequest) (*ListProjectFilesResponse, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	snapshot, err := r.getOrCreateSnapshot(ctx, req.Snapshot)
	if err != nil {
		return nil, err
	}

	projectPath := path.Join(protosDir, string(req.Project))
	entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
		Recurse: true,
		Paths:   []string{projectPath},
	})
	if err != nil {
		return nil, fmt.Errorf("read tree: %w", err)
	}

	var files []ProjectFile
	for _, entry := range entries {
		if entry.Type != git.BlobType {
			continue
		}

		// Only include .proto files
		if !strings.HasSuffix(entry.Path, protoFileExt) {
			continue
		}

		// Get relative path
		relPath := strings.TrimPrefix(entry.Path, projectPath+"/")

		files = append(files, ProjectFile{
			Snapshot: snapshot,
			Project:  req.Project,
			Path:     relPath,
			Hash:     entry.Hash,
		})
	}

	return &ListProjectFilesResponse{
		Files:    files,
		Snapshot: snapshot,
	}, nil
}

// ReadProjectFile reads a file from the registry.
func (r *Cache) ReadProjectFile(ctx context.Context, file ProjectFile, writer io.Writer) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	return r.repo.ReadObject(ctx, git.BlobType, file.Hash, writer)
}

// SetProject updates a project in the registry.
func (r *Cache) SetProject(ctx context.Context, req *SetProjectRequest) (*SetProjectResponse, error) {
	snapshot, err := r.getOrCreateSnapshot(ctx, req.Snapshot)
	if err != nil {
		return nil, err
	}

	currentTree, err := r.repo.RevHash(ctx, string(snapshot)+"^{tree}")
	if err != nil {
		return nil, fmt.Errorf("get current tree: %w", err)
	}

	projectPrefix := path.Join(protosDir, string(req.Project.Path))
	upserts, err := r.prepareUpserts(ctx, req.Project, req.Files, projectPrefix)
	if err != nil {
		return nil, err
	}

	deletes, err := r.prepareDeletes(ctx, req.Project.Path, req.Files, snapshot, projectPrefix)
	if err != nil {
		return nil, err
	}

	newTree, err := r.repo.UpdateTree(ctx, git.UpdateTreeRequest{
		Tree:    currentTree,
		Upserts: upserts,
		Deletes: deletes,
	})
	if err != nil {
		return nil, fmt.Errorf("update tree: %w", err)
	}

	newCommit, err := r.createProjectCommit(ctx, req, snapshot, newTree)
	if err != nil {
		return nil, err
	}

	return &SetProjectResponse{
		Snapshot:     newCommit,
		FilesChanged: len(req.Files),
	}, nil
}

// getOrCreateSnapshot gets the snapshot from request or creates a new one.
func (r *Cache) getOrCreateSnapshot(ctx context.Context, snapshot git.Hash) (git.Hash, error) {
	if snapshot != "" {
		return snapshot, nil
	}
	snapshot, err := r.Snapshot(ctx)
	if err != nil {
		return "", fmt.Errorf("get snapshot: %w", err)
	}
	return snapshot, nil
}

// prepareUpserts prepares tree upserts for project metadata and files.
func (r *Cache) prepareUpserts(ctx context.Context, project *Project, files []LocalProjectFile, projectPrefix string) ([]git.TreeUpsert, error) {
	var upserts []git.TreeUpsert

	// Write project metadata
	metaContent := fmt.Sprintf("git:\n  commit: %s\n  url: %s\n", project.Commit, project.RepositoryURL)
	metaHash, err := r.repo.WriteObject(ctx, strings.NewReader(metaContent), git.WriteObjectOptions{})
	if err != nil {
		return nil, fmt.Errorf("write project meta: %w", err)
	}
	upserts = append(upserts, git.TreeUpsert{
		Path: path.Join(projectPrefix, projectMetaFile),
		Blob: metaHash,
		Mode: 0100644,
	})

	// Write files
	for _, file := range files {
		var hash git.Hash
		var err error

		if file.Content != nil {
			// Use provided content (e.g., transformed imports)
			hash, err = r.repo.WriteObject(ctx, bytes.NewReader(file.Content), git.WriteObjectOptions{})
			if err != nil {
				return nil, fmt.Errorf("write transformed object: %w", err)
			}
		} else {
			// Read from local file
			f, err := os.Open(file.LocalPath)
			if err != nil {
				return nil, fmt.Errorf("open file %s: %w", file.LocalPath, err)
			}

			hash, err = r.repo.WriteObject(ctx, f, git.WriteObjectOptions{})
			f.Close()
			if err != nil {
				return nil, fmt.Errorf("write object: %w", err)
			}
		}

		upserts = append(upserts, git.TreeUpsert{
			Path: path.Join(projectPrefix, file.Path),
			Blob: hash,
			Mode: 0100644,
		})
	}

	return upserts, nil
}

// prepareDeletes prepares which files should be deleted from the registry.
func (r *Cache) prepareDeletes(ctx context.Context, projectPath ProjectPath, newFiles []LocalProjectFile, snapshot git.Hash, projectPrefix string) ([]string, error) {
	existingFiles, _ := r.ListProjectFiles(ctx, &ListProjectFilesRequest{
		Project:  projectPath,
		Snapshot: snapshot,
	})

	newFilesMap := make(map[string]bool)
	for _, f := range newFiles {
		newFilesMap[f.Path] = true
	}

	var deletes []string
	if existingFiles != nil {
		for _, f := range existingFiles.Files {
			if !newFilesMap[f.Path] {
				deletes = append(deletes, path.Join(projectPrefix, f.Path))
			}
		}
	}

	return deletes, nil
}

// createProjectCommit creates a commit for the project update.
func (r *Cache) createProjectCommit(ctx context.Context, req *SetProjectRequest, snapshot git.Hash, tree git.Hash) (git.Hash, error) {
	if req.Author == nil {
		return "", fmt.Errorf("author is required")
	}

	message := fmt.Sprintf("%s: %d files", req.Project.Path, len(req.Files))
	newCommit, err := r.repo.CommitTree(ctx, git.CommitTreeRequest{
		Tree:    tree,
		Parents: []git.Hash{snapshot},
		Message: message,
		Author:  *req.Author,
	})
	if err != nil {
		return "", fmt.Errorf("create commit: %w", err)
	}

	return newCommit, nil
}

// Push pushes a commit to the remote registry.
func (r *Cache) Push(ctx context.Context, hash git.Hash) error {
	// Get the default branch from HEAD
	branch := r.getDefaultBranch(ctx)

	return r.repo.Push(ctx, git.PushOptions{
		Remote: "origin",
		RefSpecs: []git.Refspec{
			git.Refspec(fmt.Sprintf("%s:refs/heads/%s", hash, branch)),
		},
	})
}

// getDefaultBranch returns the default branch name (main, master, etc.)
func (r *Cache) getDefaultBranch(ctx context.Context) string {
	headRef, err := r.repo.RevHash(ctx, "HEAD")
	if err != nil {
		return "main"
	}

	branch := r.findBranchMatchingHash(ctx, headRef)
	if branch != "" {
		return branch
	}

	// Default to main (more common now) if we can't detect
	return "main"
}

// findBranchMatchingHash checks common branch names to find one matching the given hash.
func (r *Cache) findBranchMatchingHash(ctx context.Context, hash git.Hash) string {
	for _, branch := range []string{"main", "master"} {
		if r.branchMatchesHash(ctx, branch, hash) {
			return branch
		}
	}
	return ""
}

// branchMatchesHash checks if a branch (local or remote) matches the given hash.
func (r *Cache) branchMatchesHash(ctx context.Context, branch string, hash git.Hash) bool {
	// Check local refs first (for bare repos after clone)
	if branchHash, err := r.repo.RevHash(ctx, "refs/heads/"+branch); err == nil {
		if hash == branchHash {
			return true
		}
	}
	// Also check remote refs (after fetch)
	if branchHash, err := r.repo.RevHash(ctx, "refs/remotes/origin/"+branch); err == nil {
		if hash == branchHash {
			return true
		}
	}
	return false
}

// URL returns the registry URL.
func (r *Cache) URL() string {
	return r.url
}
