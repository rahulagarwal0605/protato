package cmd

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/rahulagarwal0605/protato/internal/constants"
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/protoc"
	"github.com/rahulagarwal0605/protato/internal/registry"
	"github.com/rahulagarwal0605/protato/internal/utils"
)

// PushCmd publishes owned projects to registry.
type PushCmd struct {
	Retries    int           `help:"Number of retries on conflict" default:"5" env:"PROTATO_PUSH_RETRIES"`
	RetryDelay time.Duration `help:"Delay between retries" default:"200ms" env:"PROTATO_PUSH_RETRY_DELAY"`
	NoValidate bool          `help:"Skip proto validation"`
}

// pushCtx holds the context for a push operation.
type pushCtx struct {
	wctx          *WorkspaceContext
	reg           registry.CacheInterface
	repoURL       string
	currentCommit git.Hash
	ownedProjects []local.ProjectPath
	author        *git.Author // Current Git user for commits
}

// Run executes the push command.
func (c *PushCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	pctx, err := c.createPushContext(ctx, globals)
	if err != nil {
		return err
	}

	if len(pctx.ownedProjects) == 0 {
		logger.Log(ctx).Info().Msg("No owned projects to push")
		return nil
	}

	return c.executePush(ctx, pctx)
}

// createPushContext initializes all resources needed for push.
func (c *PushCmd) createPushContext(ctx context.Context, globals *GlobalOptions) (*pushCtx, error) {
	// Check registry URL first
	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return nil, err
	}

	wctx, err := OpenWorkspaceContext(ctx)
	if err != nil {
		return nil, err
	}

	ownedProjects, err := wctx.WS.OwnedProjects()
	if err != nil {
		return nil, fmt.Errorf("get owned projects: %w", err)
	}

	repoURL, err := wctx.Repo.GetRepoURL(ctx)
	if err != nil {
		return nil, err
	}

	currentCommit, err := wctx.Repo.RevHash(ctx, "HEAD")
	if err != nil {
		return nil, fmt.Errorf("get HEAD: %w", err)
	}

	// Get current Git user (required for push)
	user, err := wctx.Repo.GetUser(ctx)
	if err != nil {
		return nil, fmt.Errorf("get Git user: %w", err)
	}
	author := &user

	return &pushCtx{
		wctx:          wctx,
		reg:           reg,
		repoURL:       repoURL,
		currentCommit: currentCommit,
		ownedProjects: ownedProjects,
		author:        author,
	}, nil
}

// executePush attempts to push with optimistic locking retries.
func (c *PushCmd) executePush(ctx context.Context, pctx *pushCtx) error {
	for attempt := 1; attempt <= c.Retries+1; attempt++ {
		logger.Log(ctx).Debug().Int("attempt", attempt).Msg("Push attempt")

		err := c.attemptPush(ctx, pctx)
		if err == nil {
			return nil
		}

		// Don't retry non-retryable errors (validation, ownership, etc.)
		if !c.isRetryableError(err) {
			return err
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


// isRetryableError determines if an error should be retried.
// Returns false for validation errors, ownership errors, and other non-transient errors.
// Returns true for push conflicts and network errors that might succeed on retry.
func (c *PushCmd) isRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Non-retryable error patterns
	nonRetryablePatterns := []string{
		constants.ErrMsgValidationFailed,
		constants.ErrMsgCompilationFailed,
		constants.ErrMsgProjectClaim,
		constants.ErrMsgOwnership,
	}

	if utils.ContainsAny(errStr, nonRetryablePatterns...) {
		return false
	}

	// Push conflicts and network errors are retryable
	// (These are typically the only errors that benefit from retry)
	return true
}


// attemptPush performs a single push attempt.
func (c *PushCmd) attemptPush(ctx context.Context, pctx *pushCtx) error {
	snapshot, err := pctx.reg.RefreshAndGetSnapshot(ctx)
	if err != nil {
		return err
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
func (c *PushCmd) checkOwnershipClaims(ctx context.Context, pctx *pushCtx, snapshot git.Hash) error {
	for _, project := range pctx.ownedProjects {
		registryPath, err := pctx.wctx.WS.GetRegistryPathForProject(project)
		if err != nil {
			return err
		}
		if err := pctx.reg.CheckProjectClaim(ctx, snapshot, pctx.repoURL, string(registryPath)); err != nil {
			return err
		}
	}
	return nil
}

// updateProjects updates all owned projects in the registry.
func (c *PushCmd) updateProjects(ctx context.Context, pctx *pushCtx, snapshot git.Hash) (git.Hash, []registry.ProjectPath, error) {
	var finalSnapshot git.Hash
	var registryProjects []registry.ProjectPath

	for _, project := range pctx.ownedProjects {
		registryPath, err := pctx.wctx.WS.GetRegistryPathForProject(project)
		if err != nil {
			return "", nil, err
		}
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
func (c *PushCmd) updateSingleProject(ctx context.Context, pctx *pushCtx, localProject local.ProjectPath, registryPath local.ProjectPath, snapshot git.Hash) (git.Hash, error) {
	files, err := pctx.wctx.WS.ListOwnedProjectFiles(localProject)
	if err != nil {
		return "", fmt.Errorf("list files %s: %w", localProject, err)
	}

	ownedDir, _ := pctx.wctx.WS.OwnedDirName()
	serviceName := pctx.wctx.WS.ServiceName()
	pulledPrefixes := c.getPulledPrefixes(ctx, pctx)
	regFiles := c.prepareRegistryFiles(ctx, files, ownedDir, serviceName, pulledPrefixes)

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

// getPulledPrefixes extracts service name prefixes from pulled projects.
// These imports should just have ownedDir stripped, not get our service prefix.
func (c *PushCmd) getPulledPrefixes(ctx context.Context, pctx *pushCtx) []string {
	received, err := pctx.wctx.WS.ReceivedProjects(ctx)
	if err != nil {
		return nil
	}

	seen := make(map[string]bool)
	var pulledPrefixes []string
	for _, r := range received {
		// Extract the service name (first part) from pulled project path
		// e.g., "lcs-svc/common" -> "lcs-svc"
		parts := strings.SplitN(string(r.Project), "/", 2)
		if len(parts) > 0 && !seen[parts[0]] {
			pulledPrefixes = append(pulledPrefixes, parts[0])
			seen[parts[0]] = true
		}
	}
	return pulledPrefixes
}

// prepareRegistryFiles prepares registry files with transformed imports.
func (c *PushCmd) prepareRegistryFiles(ctx context.Context, files []local.ProjectFile, ownedDir, serviceName string, pulledPrefixes []string) []registry.LocalProjectFile {
	regFiles := make([]registry.LocalProjectFile, len(files))
	for i, f := range files {
		regFile := registry.LocalProjectFile{
			Path:      f.Path,
			LocalPath: f.AbsolutePath,
		}

		if strings.HasSuffix(f.Path, constants.ProtoFileExt) && serviceName != "" {
			transformed := c.transformProtoFile(ctx, f.AbsolutePath, f.Path, ownedDir, serviceName, pulledPrefixes)
			if transformed != nil {
				regFile.Content = transformed
			}
		}

		regFiles[i] = regFile
	}
	return regFiles
}

// transformProtoFile transforms imports in a proto file and returns the transformed content if changed.
func (c *PushCmd) transformProtoFile(ctx context.Context, filePath, fileName, ownedDir, serviceName string, pulledPrefixes []string) []byte {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return nil
	}

	transformed := protoc.TransformImportsWithPulled(content, ownedDir, serviceName, pulledPrefixes)
	if !bytes.Equal(content, transformed) {
		logger.Log(ctx).Debug().Str("file", fileName).Str("ownedDir", ownedDir).Str("serviceName", serviceName).Msg("Transformed imports")
		return transformed
	}

	logger.Log(ctx).Debug().Str("file", fileName).Str("ownedDir", ownedDir).Str("serviceName", serviceName).Msg("No imports transformed")
	return nil
}

// validateIfEnabled runs proto validation if enabled.
func (c *PushCmd) validateIfEnabled(ctx context.Context, pctx *pushCtx, snapshot git.Hash, projects []registry.ProjectPath) error {
	if c.NoValidate {
		return nil
	}

	// Get owned directory name (e.g., "proto") for import path mapping
	ownedDir, err := pctx.wctx.WS.OwnedDirName()
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Could not get owned directory, using default")
		ownedDir = "proto"
	}

	workspaceRoot := pctx.wctx.WS.Root()
	serviceName := pctx.wctx.WS.ServiceName()

	// Get vendor directory for pulled dependencies
	vendorDir, err := pctx.wctx.WS.VendorDir()
	if err != nil {
		vendorDir = "" // No vendor dir configured, that's OK
	}

	logger.Log(ctx).Info().Msg("Validating proto files")
	if err := protoc.ValidateProtos(ctx, protoc.ValidateProtosConfig{
		Cache:         pctx.reg,
		Snapshot:      snapshot,
		Projects:      projects,
		OwnedDir:      ownedDir,
		VendorDir:     vendorDir,
		WorkspaceRoot: workspaceRoot,
		ServiceName:   serviceName,
	}); err != nil {
		return fmt.Errorf("%s: %w", constants.ErrMsgValidationFailed, err)
	}

	return nil
}

// pushToRemote pushes the final snapshot to the remote registry.
func (c *PushCmd) pushToRemote(ctx context.Context, pctx *pushCtx, snapshot git.Hash) error {
	logger.Log(ctx).Info().Str("snapshot", snapshot.Short()).Msg("Pushing to registry")

	if err := pctx.reg.Push(ctx, snapshot); err != nil {
		return err
	}

	logger.Log(ctx).Info().Msg("Push complete")
	return nil
}
