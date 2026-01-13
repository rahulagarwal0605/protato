package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
	"github.com/rahulagarwal0605/protato/internal/utils"
)

// VerifyCmd verifies workspace integrity.
type VerifyCmd struct {
	Offline bool `help:"Don't refresh registry"`
}

// verifyCtx holds resources for verification.
type verifyCtx struct {
	wctx    *WorkspaceContext
	reg     registry.CacheInterface
	repoURL string
}

// Run executes the verify command.
func (c *VerifyCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	vctx, err := c.prepareverifyCtx(ctx, globals)
	if err != nil {
		return err
	}

	var hasErrors bool

	if vctx.reg != nil {
		if err := c.verifyOwnedProjects(ctx, vctx); err != nil {
			hasErrors = true
		}

		if err := c.verifyPulledProjects(ctx, vctx); err != nil {
			hasErrors = true
		}
	}

	if err := c.verifyOrphanedFiles(ctx, vctx.wctx.WS); err != nil {
		hasErrors = true
	}

	if hasErrors {
		return fmt.Errorf("verification failed")
	}

	logger.Log(ctx).Info().Msg("Verification passed")
	return nil
}

// prepareverifyCtx initializes verification resources.
func (c *VerifyCmd) prepareverifyCtx(ctx context.Context, globals *GlobalOptions) (*verifyCtx, error) {
	wctx, err := OpenWorkspaceContext(ctx)
	if err != nil {
		return nil, err
	}

	repoURL, err := wctx.Repo.GetRepoURL(ctx)
	if err != nil {
		return nil, err
	}

	var reg registry.CacheInterface
	if globals.RegistryURL != "" {
		reg, err = c.openRegistry(ctx, globals)
		if err != nil {
			logger.Log(ctx).Warn().Err(err).Msg("Failed to open registry")
		}
	}

	return &verifyCtx{
		wctx:    wctx,
		reg:     reg,
		repoURL: repoURL,
	}, nil
}

// openRegistry opens and optionally refreshes the registry.
func (c *VerifyCmd) openRegistry(ctx context.Context, globals *GlobalOptions) (registry.CacheInterface, error) {
	return OpenRegistryWithRefresh(ctx, globals, c.Offline)
}

// verifyOwnedProjects checks ownership claims for owned projects.
func (c *VerifyCmd) verifyOwnedProjects(ctx context.Context, vctx *verifyCtx) error {
	logger.Log(ctx).Info().Msg("Checking owned project claims")

	snapshot, _ := vctx.reg.Snapshot(ctx)
	ownedProjects, _ := vctx.wctx.WS.OwnedProjects()

	var hasErrors bool
	for _, project := range ownedProjects {
		if err := vctx.reg.CheckProjectClaim(ctx, snapshot, vctx.repoURL, string(project)); err != nil {
			logger.Log(ctx).Error().Str("project", string(project)).Err(err).Msg("Claim check failed")
			hasErrors = true
		} else {
			logger.Log(ctx).Debug().Str("project", string(project)).Msg("Claim OK")
		}
	}

	if hasErrors {
		return fmt.Errorf("ownership verification failed")
	}
	return nil
}

// verifyPulledProjects checks integrity of pulled projects.
func (c *VerifyCmd) verifyPulledProjects(ctx context.Context, vctx *verifyCtx) error {
	logger.Log(ctx).Info().Msg("Checking pulled project integrity")

	receivedProjects, err := vctx.wctx.WS.ReceivedProjects(ctx)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to get received projects")
		return nil
	}

	var hasErrors bool
	for _, received := range receivedProjects {
		if err := c.verifyReceivedProject(ctx, vctx, received); err != nil {
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("pulled project verification failed")
	}
	return nil
}

// verifyReceivedProject checks a single received project.
func (c *VerifyCmd) verifyReceivedProject(ctx context.Context, vctx *verifyCtx, received *local.ReceivedProject) error {
	snapshot := git.Hash(received.ProviderSnapshot)
	project := registry.ProjectPath(received.Project)

	regFiles, localFiles, err := c.getProjectFiles(ctx, vctx, project, snapshot)
	if err != nil {
		return err
	}

	regFileMap := utils.SliceToMapWithValue(regFiles, func(f registry.ProjectFile) string { return f.Path }, func(f registry.ProjectFile) git.Hash { return f.Hash })
	localFileSet := utils.BuildFileSet(localFiles, func(f local.ProjectFile) string { return f.Path })

	var hasErrors bool

	for _, f := range localFiles {
		if err := c.verifyLocalFile(ctx, vctx, project, snapshot, f, regFileMap); err != nil {
			hasErrors = true
		}
	}

	for regPath := range regFileMap {
		if !localFileSet[regPath] {
			logProjectFileError(ctx, project, regPath, "File deleted locally")
			hasErrors = true
		}
	}

	if hasErrors {
		return fmt.Errorf("project %s has local modifications", project)
	}
	return nil
}


// getProjectFiles retrieves files from both registry and local workspace.
func (c *VerifyCmd) getProjectFiles(ctx context.Context, vctx *verifyCtx, project registry.ProjectPath, snapshot git.Hash) ([]registry.ProjectFile, []local.ProjectFile, error) {
	regFiles, err := vctx.reg.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
		Project:  project,
		Snapshot: snapshot,
	})
	if err != nil {
		logProjectError(ctx, err, project, "Failed to list registry files")
		return nil, nil, err
	}

	localFiles, err := vctx.wctx.WS.ListVendorProjectFiles(local.ProjectPath(project))
	if err != nil {
		logProjectError(ctx, err, project, "Failed to list local files")
		return nil, nil, err
	}

	return regFiles.Files, localFiles, nil
}



// verifyLocalFile checks if a local file matches the registry.
func (c *VerifyCmd) verifyLocalFile(ctx context.Context, vctx *verifyCtx, project registry.ProjectPath, snapshot git.Hash, f local.ProjectFile, regFileMap map[string]git.Hash) error {
	regHash, exists := regFileMap[f.Path]
	if !exists {
		logProjectFileError(ctx, project, f.Path, "File added locally")
		return fmt.Errorf("file added: %s", f.Path)
	}

	localData, err := os.ReadFile(f.AbsolutePath)
	if err != nil {
		return nil
	}

	var regData bytes.Buffer
	if err := vctx.reg.ReadProjectFile(ctx, registry.ProjectFile{
		Snapshot: snapshot,
		Project:  project,
		Path:     f.Path,
		Hash:     regHash,
	}, &regData); err != nil {
		return nil
	}

	localHash := sha256.Sum256(localData)
	regFileHash := sha256.Sum256(regData.Bytes())

	if localHash != regFileHash {
		logProjectFileError(ctx, project, f.Path, "File modified locally")
		return fmt.Errorf("file modified: %s", f.Path)
	}

	return nil
}

// verifyOrphanedFiles checks for files not belonging to any project.
func (c *VerifyCmd) verifyOrphanedFiles(ctx context.Context, ws local.WorkspaceInterface) error {
	logger.Log(ctx).Info().Msg("Checking for orphaned files")

	orphaned, err := ws.OrphanedFiles(ctx)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to check for orphaned files")
		return nil
	}

	for _, f := range orphaned {
		logger.Log(ctx).Warn().Str("file", f).Msg("Orphaned file")
	}

	return nil
}
