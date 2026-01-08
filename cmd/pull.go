package cmd

import (
	"context"
	"fmt"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// PullCmd downloads projects from registry.
type PullCmd struct {
	Projects []string `arg:"" optional:"" help:"Projects to pull"`
	Force    bool     `help:"Force pull even if files would be deleted" short:"f"`
	NoDeps   bool     `help:"Don't pull dependencies" name:"no-deps"`
}

// pullPlan represents the plan for pulling a project.
type pullPlan struct {
	project  registry.ProjectPath
	files    []registry.ProjectFile
	toDelete []string
}

// Run executes the pull command.
func (c *PullCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	wctx, err := OpenWorkspaceContext(ctx)
	if err != nil {
		return err
	}

	reg, err := OpenAndRefreshRegistry(ctx, globals)
	if err != nil {
		return err
	}

	snapshot, err := reg.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("get snapshot: %w", err)
	}
	logger.Log(ctx).Debug().Str("snapshot", snapshot.Short()).Msg("Using registry snapshot")

	projectsToPull, err := c.resolveProjects(ctx, wctx.WS, reg, snapshot)
	if err != nil {
		return err
	}

	if len(projectsToPull) == 0 {
		logger.Log(ctx).Info().Msg("No projects to pull")
		return nil
	}

	plans, err := c.createPullPlans(ctx, wctx.WS, reg, snapshot, projectsToPull)
	if err != nil {
		return err
	}

	return c.executePullPlans(ctx, wctx.WS, reg, snapshot, plans)
}

// resolveProjects determines which projects need to be pulled.
func (c *PullCmd) resolveProjects(ctx context.Context, ws *local.Workspace, reg *registry.Cache, snapshot git.Hash) ([]registry.ProjectPath, error) {
	projectsToPull := c.getInitialProjects(ctx, ws)
	ownedPaths := c.buildOwnedPathsSet(ws)

	if !c.NoDeps && len(projectsToPull) > 0 {
		projectsToPull = c.discoverDependencies(ctx, reg, snapshot, projectsToPull)
	}

	return c.filterOwnedProjects(projectsToPull, ownedPaths), nil
}

// getInitialProjects returns the initial list of projects to pull.
func (c *PullCmd) getInitialProjects(ctx context.Context, ws *local.Workspace) []registry.ProjectPath {
	if len(c.Projects) > 0 {
		projects := make([]registry.ProjectPath, len(c.Projects))
		for i, p := range c.Projects {
			projects[i] = registry.ProjectPath(p)
		}
		return projects
	}

	received, err := ws.ReceivedProjects(ctx)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to get received projects")
		return nil
	}

	projects := make([]registry.ProjectPath, len(received))
	for i, r := range received {
		projects[i] = registry.ProjectPath(r.Project)
	}
	return projects
}

// buildOwnedPathsSet builds a set of owned project paths.
func (c *PullCmd) buildOwnedPathsSet(ws *local.Workspace) map[string]bool {
	ownedPaths := make(map[string]bool)
	ownedProjects, _ := ws.OwnedProjects()

	for _, p := range ownedProjects {
		ownedPaths[string(p)] = true
		registryPath, err := ws.RegistryProjectPath(p)
		if err != nil {
			// If service is not configured, we can't determine registry path
			// but we still add the local path to filter out exact matches
			continue
		}
		ownedPaths[string(registryPath)] = true
	}

	return ownedPaths
}

// discoverDependencies discovers and adds transitive dependencies.
func (c *PullCmd) discoverDependencies(ctx context.Context, reg *registry.Cache, snapshot git.Hash, projects []registry.ProjectPath) []registry.ProjectPath {
	logger.Log(ctx).Info().Msg("Discovering dependencies")

	allProjects, err := protoc.DiscoverDependencies(ctx, reg, snapshot, projects)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to discover dependencies")
		return projects
	}

	return allProjects
}

// filterOwnedProjects removes owned projects from the list.
func (c *PullCmd) filterOwnedProjects(projects []registry.ProjectPath, ownedPaths map[string]bool) []registry.ProjectPath {
	var filtered []registry.ProjectPath
	for _, p := range projects {
		if !ownedPaths[string(p)] {
			filtered = append(filtered, p)
		}
	}
	return filtered
}

// createPullPlans creates execution plans for each project.
func (c *PullCmd) createPullPlans(ctx context.Context, ws *local.Workspace, reg *registry.Cache, snapshot git.Hash, projects []registry.ProjectPath) ([]pullPlan, error) {
	var plans []pullPlan

	for _, project := range projects {
		plan, err := c.createProjectPlan(ctx, ws, reg, snapshot, project)
		if err != nil {
			return nil, err
		}

		if err := c.validateDeletions(ctx, plan); err != nil {
			return nil, err
		}

		plans = append(plans, plan)
	}

	return plans, nil
}

// createProjectPlan creates a pull plan for a single project.
func (c *PullCmd) createProjectPlan(ctx context.Context, ws *local.Workspace, reg *registry.Cache, snapshot git.Hash, project registry.ProjectPath) (pullPlan, error) {
	filesRes, err := reg.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
		Project:  project,
		Snapshot: snapshot,
	})
	if err != nil {
		return pullPlan{}, fmt.Errorf("list project files %s: %w", project, err)
	}

	localFiles, err := ws.ListVendorProjectFiles(local.ProjectPath(project))
	if err != nil {
		return pullPlan{}, fmt.Errorf("list local files %s: %w", project, err)
	}

	toDelete := c.findFilesToDelete(filesRes.Files, localFiles)

	return pullPlan{
		project:  project,
		files:    filesRes.Files,
		toDelete: toDelete,
	}, nil
}

// findFilesToDelete finds local files not in the registry.
func (c *PullCmd) findFilesToDelete(regFiles []registry.ProjectFile, localFiles []local.ProjectFile) []string {
	registryFileSet := make(map[string]bool)
	for _, f := range regFiles {
		registryFileSet[f.Path] = true
	}

	var toDelete []string
	for _, lf := range localFiles {
		if !registryFileSet[lf.Path] {
			toDelete = append(toDelete, lf.Path)
		}
	}

	return toDelete
}

// validateDeletions checks if deletions are allowed.
func (c *PullCmd) validateDeletions(ctx context.Context, plan pullPlan) error {
	if len(plan.toDelete) > 0 && !c.Force {
		logger.Log(ctx).Error().
			Str("project", string(plan.project)).
			Int("count", len(plan.toDelete)).
			Msg("Would delete files. Use --force to proceed")
		return fmt.Errorf("would delete %d files in %s", len(plan.toDelete), plan.project)
	}
	return nil
}

// executePullPlans executes all pull plans.
func (c *PullCmd) executePullPlans(ctx context.Context, ws *local.Workspace, reg *registry.Cache, snapshot git.Hash, plans []pullPlan) error {
	var totalChanged, totalDeleted int

	for _, plan := range plans {
		stats, err := c.executeProjectPull(ctx, ws, reg, snapshot, plan)
		if err != nil {
			return err
		}
		totalChanged += stats.FilesChanged
		totalDeleted += stats.FilesDeleted
	}

	logger.Log(ctx).Info().
		Int("projects", len(plans)).
		Int("changed", totalChanged).
		Int("deleted", totalDeleted).
		Msg("Pull complete")

	return nil
}

// executeProjectPull pulls a single project.
func (c *PullCmd) executeProjectPull(ctx context.Context, ws *local.Workspace, reg *registry.Cache, snapshot git.Hash, plan pullPlan) (*local.ReceiveStats, error) {
	logger.Log(ctx).Info().
		Str("project", string(plan.project)).
		Int("files", len(plan.files)).
		Msg("Pulling project")

	recv, err := ws.ReceiveProject(&local.ReceiveProjectRequest{
		Project:  local.ProjectPath(plan.project),
		Snapshot: snapshot,
	})
	if err != nil {
		return nil, fmt.Errorf("receive project: %w", err)
	}

	if err := c.pullFiles(ctx, reg, recv, plan.files); err != nil {
		return nil, err
	}

	c.deleteFiles(ctx, recv, plan.toDelete)

	return recv.Finish()
}

// pullFiles downloads files from the registry.
func (c *PullCmd) pullFiles(ctx context.Context, reg *registry.Cache, recv *local.ProjectReceiver, files []registry.ProjectFile) error {
	for _, file := range files {
		w, err := recv.CreateFile(file.Path)
		if err != nil {
			return fmt.Errorf("create file %s: %w", file.Path, err)
		}

		if err := reg.ReadProjectFile(ctx, file, w); err != nil {
			w.Close()
			return fmt.Errorf("read file %s: %w", file.Path, err)
		}

		if err := w.Close(); err != nil {
			return fmt.Errorf("close file %s: %w", file.Path, err)
		}
	}
	return nil
}

// deleteFiles removes files that no longer exist in the registry.
func (c *PullCmd) deleteFiles(ctx context.Context, recv *local.ProjectReceiver, toDelete []string) {
	for _, path := range toDelete {
		if err := recv.DeleteFile(path); err != nil {
			logger.Log(ctx).Warn().Err(err).Str("path", path).Msg("Failed to delete file")
		}
	}
}
