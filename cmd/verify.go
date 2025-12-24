package cmd

import (
	"bytes"
	"context"
	"crypto/sha256"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// VerifyCmd verifies workspace integrity.
type VerifyCmd struct {
	Offline bool `help:"Don't refresh registry" name:"offline"`
}

// Run executes the verify command.
func (c *VerifyCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
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

	var hasErrors bool

	// Check 1: Owned project claims
	log.Info().Msg("Checking owned project claims")
	if globals.RegistryURL != "" {
		reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
			URL: globals.RegistryURL,
		}, log)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to open registry")
		} else {
			if !c.Offline {
				if err := reg.Refresh(ctx); err != nil {
					log.Warn().Err(err).Msg("Failed to refresh registry")
				}
			}

			repoURL, _ := repo.GetRemoteURL(ctx, "origin")
			repoURL = git.NormalizeRemoteURL(repoURL)

			snapshot, _ := reg.Snapshot(ctx)
			ownedProjects, _ := ws.OwnedProjects()

			for _, project := range ownedProjects {
				if err := checkProjectClaim(ctx, reg, snapshot, repoURL, string(project), log); err != nil {
					log.Error().Str("project", string(project)).Err(err).Msg("Claim check failed")
					hasErrors = true
				} else {
					log.Debug().Str("project", string(project)).Msg("Claim OK")
				}
			}
		}
	}

	// Check 2: Pulled project modifications
	log.Info().Msg("Checking pulled project integrity")
	if globals.RegistryURL != "" {
		reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
			URL: globals.RegistryURL,
		}, log)
		if err == nil {
			receivedProjects, err := ws.ReceivedProjects()
			if err != nil {
				log.Warn().Err(err).Msg("Failed to get received projects")
			}

			for _, received := range receivedProjects {
				snapshot := git.Hash(received.ProviderSnapshot)
				project := registry.ProjectPath(received.Project)

				// List registry files at snapshot
				regFiles, err := reg.ListProjectFiles(ctx, &registry.ListProjectFilesRequest{
					Project:  project,
					Snapshot: snapshot,
				})
				if err != nil {
					log.Warn().Err(err).Str("project", string(project)).Msg("Failed to list registry files")
					continue
				}

				// List local files
				localFiles, err := ws.ListProjectFiles(local.ProjectPath(project))
				if err != nil {
					log.Warn().Err(err).Str("project", string(project)).Msg("Failed to list local files")
					continue
				}

				// Build maps
				regFileMap := make(map[string]git.Hash)
				for _, f := range regFiles.Files {
					regFileMap[f.Path] = f.Hash
				}

				localFileSet := make(map[string]bool)
				for _, f := range localFiles {
					localFileSet[f.Path] = true
				}

				// Check each file
				for _, f := range localFiles {
					regHash, exists := regFileMap[f.Path]
					if !exists {
						log.Error().
							Str("project", string(project)).
							Str("file", f.Path).
							Msg("File added locally")
						hasErrors = true
						continue
					}

					// Compare hashes
					localData, err := os.ReadFile(f.AbsolutePath)
					if err != nil {
						continue
					}

					// Read registry file
					var regData bytes.Buffer
					if err := reg.ReadProjectFile(ctx, registry.ProjectFile{
						Snapshot: snapshot,
						Project:  project,
						Path:     f.Path,
						Hash:     regHash,
					}, &regData); err != nil {
						continue
					}

					localHash := sha256.Sum256(localData)
					regFileHash := sha256.Sum256(regData.Bytes())

					if localHash != regFileHash {
						log.Error().
							Str("project", string(project)).
							Str("file", f.Path).
							Msg("File modified locally")
						hasErrors = true
					}
				}

				// Check for deleted files
				for regPath := range regFileMap {
					if !localFileSet[regPath] {
						log.Error().
							Str("project", string(project)).
							Str("file", regPath).
							Msg("File deleted locally")
						hasErrors = true
					}
				}
			}
		}
	}

	// Check 3: Orphaned files
	log.Info().Msg("Checking for orphaned files")
	orphaned, err := ws.OrphanedFiles()
	if err != nil {
		log.Warn().Err(err).Msg("Failed to check for orphaned files")
	}
	for _, f := range orphaned {
		log.Warn().Str("file", f).Msg("Orphaned file")
	}

	if hasErrors {
		return fmt.Errorf("verification failed")
	}

	log.Info().Msg("Verification passed")
	return nil
}
