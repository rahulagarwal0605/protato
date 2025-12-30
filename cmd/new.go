package cmd

import (
	"context"
	"fmt"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/local"
)

// NewCmd creates a new project (claim ownership).
type NewCmd struct {
	Paths []string `arg:"" required:"" help:"Project paths to create (e.g., team/service)"`
}

// Run executes the new command.
func (c *NewCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	if err := c.validatePaths(); err != nil {
		return err
	}

	wctx, err := OpenWorkspace(ctx, log, local.OpenOptions{})
	if err != nil {
		return err
	}

	repoURL := GetRepoURL(ctx, wctx.Repo, log)

	if err := c.checkRegistryConflicts(ctx, globals, wctx, repoURL, log); err != nil {
		return err
	}

	return c.addProjects(wctx.WS, log)
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
func (c *NewCmd) checkRegistryConflicts(ctx context.Context, globals *GlobalOptions, wctx *WorkspaceContext, repoURL string, log *zerolog.Logger) error {
	if globals.RegistryURL == "" {
		return nil
	}

	reg, err := OpenRegistry(ctx, globals, log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to open registry")
		return nil
	}

	if err := reg.Refresh(ctx); err != nil {
		log.Warn().Err(err).Msg("Failed to refresh registry")
	}

	snapshot, _ := reg.Snapshot(ctx)

	for _, p := range c.Paths {
		registryPath := wctx.WS.RegistryProjectPath(local.ProjectPath(p))
		if err := CheckProjectClaim(ctx, reg, snapshot, repoURL, string(registryPath), log); err != nil {
			return err
		}
	}

	return nil
}

// addProjects adds the projects to the workspace.
func (c *NewCmd) addProjects(ws *local.Workspace, log *zerolog.Logger) error {
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
