package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// NewCmd creates a new project (claim ownership).
type NewCmd struct {
	Paths []string `arg:"" required:"" help:"Project paths to create (e.g., team/service)"`
}

// Run executes the new command.
func (c *NewCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	// Validate project paths
	for _, p := range c.Paths {
		if err := local.ValidateProjectPath(p); err != nil {
			return fmt.Errorf("invalid project path %q: %w", p, err)
		}
	}

	// Check for overlaps in requested projects
	if err := local.ProjectsOverlap(c.Paths); err != nil {
		return err
	}

	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	// Open Git repository
	repo, err := git.Open(ctx, cwd, git.OpenOptions{}, log)
	if err != nil {
		return fmt.Errorf("open git repo: %w", err)
	}

	// Open workspace
	ws, err := local.Open(repo.Root(), local.OpenOptions{}, log)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Get repository URL
	repoURL, err := repo.GetRemoteURL(ctx, "origin")
	if err != nil {
		log.Warn().Err(err).Msg("Failed to get remote URL")
		repoURL = ""
	}
	repoURL = git.NormalizeRemoteURL(repoURL)

	// Check registry for conflicts
	if globals.RegistryURL != "" {
		reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
			URL: globals.RegistryURL,
		}, log)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to open registry")
		} else {
			// Refresh registry
			if err := reg.Refresh(ctx); err != nil {
				log.Warn().Err(err).Msg("Failed to refresh registry")
			}

			// Check each project
			snapshot, _ := reg.Snapshot(ctx)
			for _, p := range c.Paths {
				if err := checkProjectClaim(ctx, reg, snapshot, repoURL, p, log); err != nil {
					return err
				}
			}
		}
	}

	// Add projects
	projects := make([]local.ProjectPath, len(c.Paths))
	for i, p := range c.Paths {
		projects[i] = local.ProjectPath(p)
	}

	if err := ws.AddOwnedProjects(projects); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	for _, p := range c.Paths {
		log.Info().Str("project", p).Msg("Created project")
	}

	return nil
}

// checkProjectClaim checks if a project can be claimed.
func checkProjectClaim(
	ctx context.Context,
	reg *registry.Cache,
	snapshot git.Hash,
	repoURL string,
	projectPath string,
	log *zerolog.Logger,
) error {
	// Look up project in registry
	res, err := reg.LookupProject(ctx, &registry.LookupProjectRequest{
		Path:     projectPath,
		Snapshot: snapshot,
	})
	if err == registry.ErrNotFound {
		// Project doesn't exist - check for subprojects
		subprojects, _ := reg.ListProjects(ctx, &registry.ListProjectsOptions{
			Prefix:   projectPath + "/",
			Snapshot: snapshot,
		})
		if len(subprojects) > 0 {
			return fmt.Errorf("cannot create project %q: overlaps with existing projects", projectPath)
		}
		return nil // OK to claim
	}
	if err != nil {
		return fmt.Errorf("lookup project: %w", err)
	}

	// Project exists - check ownership
	if string(res.Project.Path) != projectPath {
		// Parent project exists
		return fmt.Errorf("cannot create project %q: parent project %q already exists", projectPath, res.Project.Path)
	}

	if repoURL != "" && res.Project.RepositoryURL != repoURL {
		return fmt.Errorf("project %q is owned by %s", projectPath, res.Project.RepositoryURL)
	}

	log.Info().Str("project", projectPath).Msg("Project already exists in registry, adding to local config")
	return nil
}
