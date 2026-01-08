package cmd

import (
	"context"
	"fmt"

	"github.com/rahulagarwal0605/protato/internal/local"
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

	repoURL, err := GetRepoURL(ctx, wctx.Repo)
	if err != nil {
		return err
	}

	if err := c.checkRegistryConflicts(ctx, globals, wctx, repoURL); err != nil {
		return err
	}

	if err := wctx.WS.AddOwnedProjects(c.Paths); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	return nil
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
		return fmt.Errorf("registry URL not configured")
	}

	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}

	if err := reg.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh registry: %w", err)
	}

	snapshot, err := reg.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("get snapshot: %w", err)
	}

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
