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

	"github.com/rs/zerolog"
	"gopkg.in/yaml.v3"

	"github.com/rahulagarwal0605/protato/internal/git"
)

const (
	registryConfigFile = "protato.registry.yaml"
	projectMetaFile    = "protato.root.yaml"
	protosDir          = "protos"
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
	root      string // Cache directory path
	log       *zerolog.Logger
	repo      gitRepository // Bare Git repository
	ignores   []string      // Registry-level ignores
	committer RegistryCommitter
	url       string // Registry URL
}

// Open opens or initializes the registry cache.
func Open(ctx context.Context, cacheDir string, cfg Config, log *zerolog.Logger) (*Cache, error) {
	// Create cache directory hash from URL
	urlHash := sha256.Sum256([]byte(cfg.URL))
	cacheRoot := filepath.Join(cacheDir, fmt.Sprintf("%x", urlHash[:8]))

	var repo *git.Repository
	var err error

	// Check if cache exists
	if _, statErr := os.Stat(cacheRoot); os.IsNotExist(statErr) {
		// Clone the repository
		log.Info().Str("url", cfg.URL).Msg("Cloning registry")
		repo, err = git.Clone(ctx, cfg.URL, cacheRoot, git.CloneOptions{
			Bare:   true,
			NoTags: true,
			Depth:  1,
		}, log)
		if err != nil {
			return nil, fmt.Errorf("clone registry: %w", err)
		}
	} else {
		// Open existing cache
		repo, err = git.Open(ctx, cacheRoot, git.OpenOptions{Bare: true}, log)
		if err != nil {
			return nil, fmt.Errorf("open registry cache: %w", err)
		}
	}

	cache := &Cache{
		root: cacheRoot,
		log:  log,
		repo: repo,
		url:  cfg.URL,
		committer: RegistryCommitter{
			Name:  "Protato Bot",
			Email: "protato@example.com",
		},
	}

	// Load registry config if exists
	if err := cache.loadConfig(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to load registry config")
	}

	return cache, nil
}

// loadConfig loads the registry configuration.
func (r *Cache) loadConfig(ctx context.Context) error {
	snapshot, err := r.Snapshot(ctx)
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := r.repo.ReadObject(ctx, git.BlobType, snapshot, &buf); err != nil {
		// Try reading config file
		entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
			Paths: []string{registryConfigFile},
		})
		if err != nil || len(entries) == 0 {
			return nil // No config file
		}

		if err := r.repo.ReadObject(ctx, git.BlobType, entries[0].Hash, &buf); err != nil {
			return err
		}
	}

	var cfg Config
	if err := yaml.Unmarshal(buf.Bytes(), &cfg); err != nil {
		return err
	}

	r.ignores = cfg.Ignores
	if cfg.Committer.Name != "" {
		r.committer = cfg.Committer
	}

	return nil
}

// Refresh refreshes the cache from remote.
func (r *Cache) Refresh(ctx context.Context) error {
	r.log.Debug().Msg("Refreshing registry cache")
	return r.repo.Fetch(ctx, git.FetchOptions{
		Remote: "origin",
		Depth:  1,
		Prune:  true,
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
	snapshot := req.Snapshot
	if snapshot == "" {
		var err error
		snapshot, err = r.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
	}

	// Verify snapshot exists
	if !r.repo.RevExists(ctx, string(snapshot)) {
		return nil, fmt.Errorf("snapshot not found: %s", snapshot)
	}

	// Search for project root file
	projectPath := req.Path
	for {
		metaPath := path.Join(protosDir, projectPath, projectMetaFile)
		entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
			Paths: []string{metaPath},
		})
		if err == nil && len(entries) > 0 {
			// Found project root
			project, err := r.readProjectMeta(ctx, entries[0].Hash)
			if err != nil {
				return nil, err
			}
			project.Path = ProjectPath(projectPath)

			// Get project tree hash
			projTreePath := path.Join(protosDir, projectPath)
			treeEntries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
				Paths: []string{projTreePath},
			})
			var projectHash git.Hash
			if err == nil && len(treeEntries) > 0 {
				projectHash = treeEntries[0].Hash
			}

			return &LookupProjectResponse{
				Project:     project,
				Snapshot:    snapshot,
				ProjectHash: projectHash,
			}, nil
		}

		// Move up one level
		parent := path.Dir(projectPath)
		if parent == "." || parent == projectPath {
			break
		}
		projectPath = parent
	}

	return nil, ErrNotFound
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
	if snapshot == "" {
		var err error
		snapshot, err = r.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
	}

	// List all files in protos/
	entries, err := r.repo.ReadTree(ctx, git.Treeish(snapshot), git.ReadTreeOptions{
		Recurse: true,
		Paths:   []string{protosDir},
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

		// Apply prefix filter
		if opts != nil && opts.Prefix != "" {
			if !strings.HasPrefix(projectPath, opts.Prefix) {
				continue
			}
		}

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
	snapshot := req.Snapshot
	if snapshot == "" {
		var err error
		snapshot, err = r.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
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

		// Skip metadata files
		name := path.Base(entry.Path)
		if name == projectMetaFile || name == ".gitattributes" {
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
	return r.repo.ReadObject(ctx, git.BlobType, file.Hash, writer)
}

// SetProject updates a project in the registry.
func (r *Cache) SetProject(ctx context.Context, req *SetProjectRequest) (*SetProjectResponse, error) {
	snapshot := req.Snapshot
	if snapshot == "" {
		var err error
		snapshot, err = r.Snapshot(ctx)
		if err != nil {
			return nil, fmt.Errorf("get snapshot: %w", err)
		}
	}

	// Get current tree
	currentTree, err := r.repo.RevHash(ctx, string(snapshot)+"^{tree}")
	if err != nil {
		return nil, fmt.Errorf("get current tree: %w", err)
	}

	// Prepare upserts
	var upserts []git.TreeUpsert
	projectPrefix := path.Join(protosDir, string(req.Project.Path))

	// Write project metadata
	metaContent := fmt.Sprintf("git:\n  commit: %s\n  url: %s\n", req.Project.Commit, req.Project.RepositoryURL)
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
	for _, file := range req.Files {
		f, err := os.Open(file.LocalPath)
		if err != nil {
			return nil, fmt.Errorf("open file %s: %w", file.LocalPath, err)
		}

		hash, err := r.repo.WriteObject(ctx, f, git.WriteObjectOptions{})
		f.Close()
		if err != nil {
			return nil, fmt.Errorf("write object: %w", err)
		}

		upserts = append(upserts, git.TreeUpsert{
			Path: path.Join(projectPrefix, file.Path),
			Blob: hash,
			Mode: 0100644,
		})
	}

	// Get list of files to delete (files in registry not in request)
	existingFiles, _ := r.ListProjectFiles(ctx, &ListProjectFilesRequest{
		Project:  req.Project.Path,
		Snapshot: snapshot,
	})

	newFiles := make(map[string]bool)
	for _, f := range req.Files {
		newFiles[f.Path] = true
	}

	var deletes []string
	if existingFiles != nil {
		for _, f := range existingFiles.Files {
			if !newFiles[f.Path] {
				deletes = append(deletes, path.Join(projectPrefix, f.Path))
			}
		}
	}

	// Update tree
	newTree, err := r.repo.UpdateTree(ctx, git.UpdateTreeRequest{
		Tree:    currentTree,
		Upserts: upserts,
		Deletes: deletes,
	})
	if err != nil {
		return nil, fmt.Errorf("update tree: %w", err)
	}

	// Create commit
	message := fmt.Sprintf("%s: %d files", req.Project.Path, len(req.Files))
	newCommit, err := r.repo.CommitTree(ctx, git.CommitTreeRequest{
		Tree:    newTree,
		Parents: []git.Hash{snapshot},
		Message: message,
		Author: git.Author{
			Name:  r.committer.Name,
			Email: r.committer.Email,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("create commit: %w", err)
	}

	return &SetProjectResponse{
		Snapshot:     newCommit,
		FilesChanged: len(req.Files),
	}, nil
}

// Push pushes a commit to the remote registry.
func (r *Cache) Push(ctx context.Context, hash git.Hash) error {
	return r.repo.Push(ctx, git.PushOptions{
		Remote: "origin",
		RefSpecs: []git.Refspec{
			git.Refspec(fmt.Sprintf("%s:refs/heads/main", hash)),
		},
	})
}

// URL returns the registry URL.
func (r *Cache) URL() string {
	return r.url
}
