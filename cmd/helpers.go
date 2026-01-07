package cmd

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// WorkspaceContext holds the common resources for workspace operations.
type WorkspaceContext struct {
	Repo *git.Repository
	WS   *local.Workspace
}

// FindRepoRoot finds the Git repository root directory from the current working directory.
func FindRepoRoot(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	repo, err := git.Open(ctx, cwd, git.OpenOptions{})
	if err != nil {
		return "", fmt.Errorf("open git repo: %w", err)
	}

	return repo.Root(), nil
}

// OpenWorkspace opens the Git repository and workspace from the current directory.
func OpenWorkspace(ctx context.Context, opts local.OpenOptions) (*WorkspaceContext, error) {
	root, err := FindRepoRoot(ctx)
	if err != nil {
		return nil, err
	}

	repo, err := git.Open(ctx, root, git.OpenOptions{})
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}

	ws, err := local.Open(ctx, root, opts)
	if err != nil {
		return nil, fmt.Errorf("open workspace: %w", err)
	}

	return &WorkspaceContext{
		Repo: repo,
		WS:   ws,
	}, nil
}

// OpenRegistry opens the registry cache.
func OpenRegistry(ctx context.Context, globals *GlobalOptions) (*registry.Cache, error) {
	if globals.RegistryURL == "" {
		return nil, fmt.Errorf("registry URL not configured")
	}

	reg, err := registry.Open(ctx, globals.CacheDir, globals.RegistryURL)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}

	return reg, nil
}

// OpenAndRefreshRegistry opens and refreshes the registry.
func OpenAndRefreshRegistry(ctx context.Context, globals *GlobalOptions) (*registry.Cache, error) {
	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return nil, err
	}

	logger.Log(ctx).Info().Msg("Refreshing registry")
	if err := reg.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("refresh registry: %w", err)
	}

	return reg, nil
}

// GetRepoURL returns the normalized remote URL for the repository.
func GetRepoURL(ctx context.Context, repo *git.Repository) string {
	repoURL, err := repo.GetRemoteURL(ctx, "origin")
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to get remote URL")
		return ""
	}
	return git.NormalizeRemoteURL(repoURL)
}

// CheckProjectClaim checks if a project can be claimed by the given repository.
func CheckProjectClaim(
	ctx context.Context,
	reg *registry.Cache,
	snapshot git.Hash,
	repoURL string,
	projectPath string,
) error {
	res, err := reg.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     projectPath,
		Snapshot: snapshot,
	})

	if err == registry.ErrNotFound {
		return checkSubprojectConflicts(ctx, reg, snapshot, projectPath)
	}
	if err != nil {
		return fmt.Errorf("lookup project: %w", err)
	}

	return validateOwnership(ctx, res, repoURL, projectPath)
}

// checkSubprojectConflicts checks if any subprojects exist under the path.
func checkSubprojectConflicts(ctx context.Context, reg *registry.Cache, snapshot git.Hash, projectPath string) error {
	subprojects, _ := reg.ListProjects(ctx, &registry.ListProjectsOptions{
		Prefix:   projectPath + "/",
		Snapshot: snapshot,
	})
	if len(subprojects) > 0 {
		return fmt.Errorf("cannot create project %q: overlaps with existing projects", projectPath)
	}
	return nil
}

// validateOwnership validates project ownership.
func validateOwnership(ctx context.Context, res *registry.LookupProjectResponse, repoURL, projectPath string) error {
	if string(res.Project.Path) != projectPath {
		return fmt.Errorf("cannot create project %q: parent project %q already exists", projectPath, res.Project.Path)
	}

	if repoURL != "" && res.Project.RepositoryURL != repoURL {
		return fmt.Errorf("project %q is owned by %s", projectPath, res.Project.RepositoryURL)
	}

	logger.Log(ctx).Info().Str("project", projectPath).Msg("Project already exists in registry, adding to local config")
	return nil
}

// ParseCommaSeparated parses a comma-separated string into a slice of trimmed, non-empty strings.
func ParseCommaSeparated(input string) []string {
	var result []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ExtractLiteralPaths filters a slice of patterns to return only literal paths (no glob patterns).
// A pattern is considered a glob if it contains '*' or '?' characters.
func ExtractLiteralPaths(patterns []string) []string {
	var literalPaths []string
	for _, pattern := range patterns {
		// Check if pattern contains glob characters
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
			literalPaths = append(literalPaths, pattern)
		}
	}
	return literalPaths
}
