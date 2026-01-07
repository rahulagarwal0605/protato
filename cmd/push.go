package cmd

import (
	"context"
	"fmt"
	"time"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// PushCmd publishes owned projects to registry.
type PushCmd struct {
	Retries    int           `help:"Number of retries on conflict" default:"5" env:"PROTATO_PUSH_RETRIES"`
	RetryDelay time.Duration `help:"Delay between retries" default:"200ms" env:"PROTATO_PUSH_RETRY_DELAY"`
	NoValidate bool          `help:"Skip proto validation" name:"no-validate"`
}

// pushContext holds the context for a push operation.
type pushContext struct {
	wctx          *WorkspaceContext
	reg           *registry.Cache
	repoURL       string
	currentCommit git.Hash
	ownedProjects []local.ProjectPath
	author        *git.Author // Current Git user for commits
}

// Run executes the push command.
func (c *PushCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	pctx, err := c.preparePushContext(ctx, globals)
	if err != nil {
		return err
	}

	if len(pctx.ownedProjects) == 0 {
		logger.Log(ctx).Info().Msg("No owned projects to push")
		return nil
	}

	return c.pushWithRetries(ctx, pctx)
}

// preparePushContext initializes all resources needed for push.
func (c *PushCmd) preparePushContext(ctx context.Context, globals *GlobalOptions) (*pushContext, error) {
	wctx, err := OpenWorkspace(ctx, local.OpenOptions{})
	if err != nil {
		return nil, err
	}

	ownedProjects, err := wctx.WS.OwnedProjects()
	if err != nil {
		return nil, fmt.Errorf("get owned projects: %w", err)
	}

	currentCommit, err := wctx.Repo.RevHash(ctx, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	repoURL := GetRepoURL(ctx, wctx.Repo)
	if repoURL == "" {
		return nil, fmt.Errorf("failed to get remote URL")
	}

	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return nil, err
	}

	// Get current Git user (works for both GitHub Actions and local)
	var author *git.Author
	user, err := wctx.Repo.GetUser(ctx)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to get Git user, will use registry committer")
		// Continue without author - will use registry committer as fallback
	} else {
		author = &user
	}

	return &pushContext{
		wctx:          wctx,
		reg:           reg,
		repoURL:       repoURL,
		currentCommit: currentCommit,
		ownedProjects: ownedProjects,
		author:        author,
	}, nil
}

// pushWithRetries attempts to push with optimistic locking retries.
func (c *PushCmd) pushWithRetries(ctx context.Context, pctx *pushContext) error {
	for attempt := 1; attempt <= c.Retries+1; attempt++ {
		logger.Log(ctx).Debug().Int("attempt", attempt).Msg("Push attempt")

		err := c.attemptPush(ctx, pctx)
		if err == nil {
			return nil
		}

		if attempt < c.Retries+1 {
			logger.Log(ctx).Warn().Err(err).Msg("Push failed, retrying")
			time.Sleep(c.RetryDelay * time.Duration(attempt))
			continue
		}

		return fmt.Errorf("push failed after %d attempts: %w", attempt, err)
	}

	return fmt.Errorf("push failed after %d retries", c.Retries)
}

// attemptPush performs a single push attempt.
func (c *PushCmd) attemptPush(ctx context.Context, pctx *pushContext) error {
	if err := pctx.reg.Refresh(ctx); err != nil {
		return fmt.Errorf("refresh registry: %w", err)
	}

	snapshot, err := pctx.reg.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("get snapshot: %w", err)
	}

	if err := c.checkOwnershipClaims(ctx, pctx, snapshot); err != nil {
		return err
	}

	finalSnapshot, registryProjects, err := c.updateProjects(ctx, pctx, snapshot)
	if err != nil {
		return err
	}

	if finalSnapshot == "" {
		return nil
	}

	if err := c.validateIfEnabled(ctx, pctx, finalSnapshot, registryProjects); err != nil {
		return err
	}

	return c.pushToRemote(ctx, pctx, finalSnapshot)
}

// checkOwnershipClaims verifies all projects can be pushed.
func (c *PushCmd) checkOwnershipClaims(ctx context.Context, pctx *pushContext, snapshot git.Hash) error {
	for _, project := range pctx.ownedProjects {
		registryPath := pctx.wctx.WS.RegistryProjectPath(project)
		if err := CheckProjectClaim(ctx, pctx.reg, snapshot, pctx.repoURL, string(registryPath)); err != nil {
			return err
		}
	}
	return nil
}

// updateProjects updates all owned projects in the registry.
func (c *PushCmd) updateProjects(ctx context.Context, pctx *pushContext, snapshot git.Hash) (git.Hash, []registry.ProjectPath, error) {
	var finalSnapshot git.Hash
	var registryProjects []registry.ProjectPath

	for _, project := range pctx.ownedProjects {
		registryPath := pctx.wctx.WS.RegistryProjectPath(project)
		registryProjects = append(registryProjects, registry.ProjectPath(registryPath))

		logger.Log(ctx).Info().
			Str("local", string(project)).
			Str("registry", string(registryPath)).
			Msg("Preparing project")

		newSnapshot, err := c.updateSingleProject(ctx, pctx, project, registryPath, snapshot)
		if err != nil {
			return "", nil, err
		}

		finalSnapshot = newSnapshot
		snapshot = finalSnapshot
	}

	return finalSnapshot, registryProjects, nil
}

// updateSingleProject updates a single project in the registry.
func (c *PushCmd) updateSingleProject(ctx context.Context, pctx *pushContext, localProject local.ProjectPath, registryPath local.ProjectPath, snapshot git.Hash) (git.Hash, error) {
	files, err := pctx.wctx.WS.ListOwnedProjectFiles(localProject)
	if err != nil {
		return "", fmt.Errorf("list files %s: %w", localProject, err)
	}

	regFiles := make([]registry.LocalProjectFile, len(files))
	for i, f := range files {
		regFiles[i] = registry.LocalProjectFile{
			Path:      f.Path,
			LocalPath: f.AbsolutePath,
		}
	}

	res, err := pctx.reg.SetProject(ctx, &registry.SetProjectRequest{
		Project: &registry.Project{
			Path:          registry.ProjectPath(registryPath),
			Commit:        pctx.currentCommit,
			RepositoryURL: pctx.repoURL,
		},
		Files:    regFiles,
		Snapshot: snapshot,
		Author:   pctx.author,
	})
	if err != nil {
		return "", fmt.Errorf("set project %s: %w", registryPath, err)
	}

	return res.Snapshot, nil
}

// validateIfEnabled runs proto validation if enabled.
func (c *PushCmd) validateIfEnabled(ctx context.Context, pctx *pushContext, snapshot git.Hash, projects []registry.ProjectPath) error {
	if c.NoValidate {
		return nil
	}

	logger.Log(ctx).Info().Msg("Validating proto files")
	if err := protoc.ValidateProtos(ctx, pctx.reg, snapshot, projects); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	return nil
}

// pushToRemote pushes the final snapshot to the remote registry.
func (c *PushCmd) pushToRemote(ctx context.Context, pctx *pushContext, snapshot git.Hash) error {
	logger.Log(ctx).Info().Str("snapshot", snapshot.Short()).Msg("Pushing to registry")

	if err := pctx.reg.Push(ctx, snapshot); err != nil {
		return err
	}

	logger.Log(ctx).Info().Msg("Push complete")
	return nil
}
