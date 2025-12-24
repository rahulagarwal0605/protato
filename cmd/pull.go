package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// PullCmd downloads projects from registry.
type PullCmd struct {
	Projects []string `arg:"" optional:"" help:"Projects to pull"`
	Force    bool     `help:"Force pull even if files would be deleted" short:"f"`
	NoDeps   bool     `help:"Don't pull dependencies" name:"no-deps"`
}

// Run executes the pull command.
func (c *PullCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	if globals.RegistryURL == "" {
		return fmt.Errorf("registry URL not configured")
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
	ws, err := local.Open(repo.Root(), local.OpenOptions{CreateIfMissing: true}, log)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Open registry
	reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
		URL: globals.RegistryURL,
	}, log)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}

	// Refresh registry
	log.Info().Msg("Refreshing registry")
	if err := reg.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh registry: %w", err)
	}

	// Get snapshot
	snapshot, err := reg.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("get snapshot: %w", err)
	}
	log.Debug().Str("snapshot", snapshot.Short()).Msg("Using registry snapshot")

	// Determine projects to pull
	var projectsToPull []registry.ProjectPath
	if len(c.Projects) > 0 {
		for _, p := range c.Projects {
			projectsToPull = append(projectsToPull, registry.ProjectPath(p))
		}
	} else {
		// Pull all received projects
		received, err := ws.ReceivedProjects()
		if err != nil {
			return fmt.Errorf("get received projects: %w", err)
		}
		for _, r := range received {
			projectsToPull = append(projectsToPull, registry.ProjectPath(r.Project))
		}
		if len(projectsToPull) == 0 {
			log.Info().Msg("No projects to pull")
			return nil
		}
	}

	// Phase 1: Discover dependencies
	if !c.NoDeps {
		log.Info().Msg("Discovering dependencies")
		allProjects, err := protoc.DiscoverDependencies(ctx, reg, snapshot, projectsToPull, log)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to discover dependencies")
		} else {
			// Filter out owned projects
			var newProjects []registry.ProjectPath
			for _, p := range allProjects {
				if !ws.IsProjectOwned(local.ProjectPath(p)) {
					newProjects = append(newProjects, p)
				}
			}
			projectsToPull = newProjects
		}
	}

	// Phase 2: Plan changes
	type pullPlan struct {
		project  registry.ProjectPath
		files    []registry.ProjectFile
		toDelete []string
	}
	var plans []pullPlan

	for _, project := range projectsToPull {
		// Skip owned projects
		if ws.IsProjectOwned(local.ProjectPath(project)) {
			log.Debug().Str("project", string(project)).Msg("Skipping owned project")
			continue
		}

		// List files in registry
		filesRes, err := reg.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
			Project:  project,
			Snapshot: snapshot,
		})
		if err != nil {
			return fmt.Errorf("list project files %s: %w", project, err)
		}

		// List local files
		localFiles, err := ws.ListProjectFiles(local.ProjectPath(project))
		if err != nil {
			return fmt.Errorf("list local files %s: %w", project, err)
		}

		// Build set of registry files
		registryFileSet := make(map[string]bool)
		for _, f := range filesRes.Files {
			registryFileSet[f.Path] = true
		}

		// Find files to delete
		var toDelete []string
		for _, lf := range localFiles {
			if !registryFileSet[lf.Path] {
				toDelete = append(toDelete, lf.Path)
			}
		}

		if len(toDelete) > 0 && !c.Force {
			log.Error().
				Str("project", string(project)).
				Int("count", len(toDelete)).
				Msg("Would delete files. Use --force to proceed")
			return fmt.Errorf("would delete %d files in %s", len(toDelete), project)
		}

		plans = append(plans, pullPlan{
			project:  project,
			files:    filesRes.Files,
			toDelete: toDelete,
		})
	}

	// Phase 3: Execute
	var totalChanged, totalDeleted int
	for _, plan := range plans {
		log.Info().
			Str("project", string(plan.project)).
			Int("files", len(plan.files)).
			Msg("Pulling project")

		recv := ws.ReceiveProject(&local.ReceiveProjectRequest{
			Project:  local.ProjectPath(plan.project),
			Snapshot: snapshot,
		})

		// Pull files
		for _, file := range plan.files {
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

		// Delete files
		for _, path := range plan.toDelete {
			if err := recv.DeleteFile(path); err != nil {
				log.Warn().Err(err).Str("path", path).Msg("Failed to delete file")
			}
		}

		// Finish
		stats, err := recv.Finish()
		if err != nil {
			return fmt.Errorf("finish receive: %w", err)
		}

		totalChanged += stats.FilesChanged
		totalDeleted += stats.FilesDeleted
	}

	log.Info().
		Int("projects", len(plans)).
		Int("changed", totalChanged).
		Int("deleted", totalDeleted).
		Msg("Pull complete")

	return nil
}
