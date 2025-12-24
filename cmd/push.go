package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// PushCmd publishes owned projects to registry.
type PushCmd struct {
	Retries    int           `help:"Number of retries on conflict" default:"5" env:"PROTATO_PUSH_RETRIES"`
	RetryDelay time.Duration `help:"Delay between retries" default:"200ms" env:"PROTATO_PUSH_RETRY_DELAY"`
	NoValidate bool          `help:"Skip proto validation" name:"no-validate"`
}

// Run executes the push command.
func (c *PushCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
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
	ws, err := local.Open(repo.Root(), local.OpenOptions{}, log)
	if err != nil {
		return fmt.Errorf("open workspace: %w", err)
	}

	// Get owned projects
	ownedProjects, err := ws.OwnedProjects()
	if err != nil {
		return fmt.Errorf("get owned projects: %w", err)
	}
	if len(ownedProjects) == 0 {
		log.Info().Msg("No owned projects to push")
		return nil
	}

	// Get current commit
	currentCommit, err := repo.RevHash(ctx, "HEAD")
	if err != nil {
		return fmt.Errorf("get HEAD: %w", err)
	}

	// Get repository URL
	repoURL, err := repo.GetRemoteURL(ctx, "origin")
	if err != nil {
		return fmt.Errorf("get remote URL: %w", err)
	}
	repoURL = git.NormalizeRemoteURL(repoURL)

	// Open registry
	reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
		URL: globals.RegistryURL,
	}, log)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}

	// Push with retries
	for attempt := 1; attempt <= c.Retries+1; attempt++ {
		log.Debug().Int("attempt", attempt).Msg("Push attempt")

		// Refresh registry
		if err := reg.Refresh(ctx); err != nil {
			return fmt.Errorf("refresh registry: %w", err)
		}

		snapshot, err := reg.Snapshot(ctx)
		if err != nil {
			return fmt.Errorf("get snapshot: %w", err)
		}

		// Check ownership claims
		for _, project := range ownedProjects {
			if err := checkProjectClaim(ctx, reg, snapshot, repoURL, string(project), log); err != nil {
				return err
			}
		}

		// Prepare updates
		var finalSnapshot git.Hash
		for _, project := range ownedProjects {
			log.Info().Str("project", string(project)).Msg("Preparing project")

			// List project files
			files, err := ws.ListProjectFiles(project)
			if err != nil {
				return fmt.Errorf("list files %s: %w", project, err)
			}

			// Convert to registry format
			var regFiles []registry.LocalProjectFile
			for _, f := range files {
				regFiles = append(regFiles, registry.LocalProjectFile{
					Path:      f.Path,
					LocalPath: f.AbsolutePath,
				})
			}

			// Update project
			res, err := reg.SetProject(ctx, &registry.SetProjectRequest{
				Project: &registry.Project{
					Path:          registry.ProjectPath(project),
					Commit:        currentCommit,
					RepositoryURL: repoURL,
				},
				Files:    regFiles,
				Snapshot: snapshot,
			})
			if err != nil {
				return fmt.Errorf("set project %s: %w", project, err)
			}

			finalSnapshot = res.Snapshot
			snapshot = finalSnapshot // Chain commits
		}

		// Validate if enabled
		if !c.NoValidate && finalSnapshot != "" {
			log.Info().Msg("Validating proto files")
			regProjects := make([]registry.ProjectPath, len(ownedProjects))
			for i, p := range ownedProjects {
				regProjects[i] = registry.ProjectPath(p)
			}
			if err := protoc.ValidateProtos(ctx, reg, finalSnapshot, regProjects, log); err != nil {
				return fmt.Errorf("validation failed: %w", err)
			}
		}

		// Push
		if finalSnapshot != "" {
			log.Info().Str("snapshot", finalSnapshot.Short()).Msg("Pushing to registry")
			err = reg.Push(ctx, finalSnapshot)
			if err == nil {
				log.Info().Msg("Push complete")
				return nil
			}

			log.Warn().Err(err).Msg("Push failed, retrying")
			if attempt < c.Retries+1 {
				time.Sleep(c.RetryDelay * time.Duration(attempt))
				continue
			}
		}

		return fmt.Errorf("push failed after %d attempts", attempt)
	}

	return fmt.Errorf("push failed after %d retries", c.Retries)
}
