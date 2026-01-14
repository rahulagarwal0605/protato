package cmd

import (
	"context"
	"fmt"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/utils"
)

// NewCmd creates a new project (claim ownership).
type NewCmd struct {
	Paths []string `arg:"" required:"" help:"Project paths to create (e.g., team/service)"`
}

// Run executes the new command.
func (c *NewCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	if err := c.validatePaths(); err != nil {
		return err
	}

	wctx, err := OpenWorkspaceContext(ctx)
	if err != nil {
		return err
	}

	repoURL, err := wctx.Repo.GetRepoURL(ctx)
	if err != nil {
		return err
	}

	if err := c.checkRegistryConflicts(ctx, globals, wctx, repoURL); err != nil {
		return err
	}

	if err := wctx.WS.AddOwnedProjects(c.Paths); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	logProjectCreationSuccess(ctx, wctx, c.Paths)

	return nil
}


// logProjectCreationSuccess logs success messages for each created project.
func logProjectCreationSuccess(ctx context.Context, wctx *WorkspaceContext, paths []string) {
	for _, p := range paths {
		log := logger.Log(ctx).Info().Str("project", p)
		if registryPath, err := wctx.WS.GetRegistryPath(p); err == nil {
			log = log.Str("registry_path", string(registryPath))
		}
		log.Msg("Project created successfully")
	}
}

// validatePaths validates all project paths.
func (c *NewCmd) validatePaths() error {
	for _, p := range c.Paths {
		if err := utils.ValidateProjectPath(p); err != nil {
			return fmt.Errorf("invalid project path %q: %w", p, err)
		}
	}
	return utils.ProjectsOverlap(c.Paths)
}

// checkRegistryConflicts verifies that the projects can be claimed.
func (c *NewCmd) checkRegistryConflicts(ctx context.Context, globals *GlobalOptions, wctx *WorkspaceContext, repoURL string) error {
	reg, err := OpenAndRefreshRegistry(ctx, globals)
	if err != nil {
		return err
	}

	snapshot, err := reg.GetSnapshot(ctx)
	if err != nil {
		return err
	}

	for _, p := range c.Paths {
		registryPath, err := wctx.WS.GetRegistryPath(p)
		if err != nil {
			return fmt.Errorf("get registry path for %s: %w", p, err)
		}
		if err := reg.CheckProjectClaim(ctx, snapshot, repoURL, string(registryPath)); err != nil {
			return err
		}
	}

	return nil
}
