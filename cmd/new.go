package cmd

import (
	"context"
	"fmt"

	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
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

	repoURL := GetRepoURL(ctx, wctx.Repo)

	if err := c.checkRegistryConflicts(ctx, globals, wctx, repoURL); err != nil {
		return err
	}

	return c.addProjects(ctx, wctx.WS)
}

// validatePaths validates all project paths.
func (c *NewCmd) validatePaths() error {
	for _, p := range c.Paths {
		if err := local.ValidateProjectPath(p); err != nil {
			return fmt.Errorf("invalid project path %q: %w", p, err)
		}
	}
	return local.ProjectsOverlap(c.Paths)
}

// checkRegistryConflicts verifies that the projects can be claimed.
func (c *NewCmd) checkRegistryConflicts(ctx context.Context, globals *GlobalOptions, wctx *WorkspaceContext, repoURL string) error {
	if globals.RegistryURL == "" {
		return nil
	}

	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to open registry")
		return nil
	}

	if err := reg.Refresh(ctx); err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to refresh registry")
	}

	snapshot, _ := reg.Snapshot(ctx)

	for _, p := range c.Paths {
		registryPath, err := wctx.WS.RegistryProjectPath(local.ProjectPath(p))
		if err != nil {
			return fmt.Errorf("get registry path for %s: %w", p, err)
		}
		if err := CheckProjectClaim(ctx, reg, snapshot, repoURL, string(registryPath)); err != nil {
			return err
		}
	}

	return nil
}

// addProjects adds the projects to the workspace.
func (c *NewCmd) addProjects(ctx context.Context, ws *local.Workspace) error {
	projects := make([]local.ProjectPath, len(c.Paths))
	for i, p := range c.Paths {
		projects[i] = local.ProjectPath(p)
	}

	if err := ws.AddOwnedProjects(projects); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	return nil
}
