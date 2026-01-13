package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// WorkspaceContext holds the common resources for workspace operations.
type WorkspaceContext struct {
	Repo git.RepositoryInterface
	WS   local.WorkspaceInterface
}

// GetCurrentRepo opens the Git repository from the current working directory.
func GetCurrentRepo(ctx context.Context) (git.RepositoryInterface, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("get cwd: %w", err)
	}

	repo, err := git.Open(ctx, cwd, git.OpenOptions{Bare: false})
	if err != nil {
		return nil, fmt.Errorf("open git repo: %w", err)
	}

	return repo, nil
}

// OpenWorkspaceContext opens the Git repository and workspace from the current directory.
func OpenWorkspaceContext(ctx context.Context) (*WorkspaceContext, error) {
	repo, err := GetCurrentRepo(ctx)
	if err != nil {
		return nil, err
	}

	ws, err := local.Open(ctx, repo.Root())
	if err != nil {
		return nil, fmt.Errorf("open workspace: %w", err)
	}

	return &WorkspaceContext{
		Repo: repo,
		WS:   ws,
	}, nil
}

// OpenRegistry opens the registry cache.
func OpenRegistry(ctx context.Context, globals *GlobalOptions) (registry.CacheInterface, error) {
	if globals.RegistryURL == "" {
		return nil, fmt.Errorf("registry URL not configured")
	}

	reg, err := registry.Open(ctx, globals.CacheDir, globals.RegistryURL)
	if err != nil {
		return nil, fmt.Errorf("open registry: %w", err)
	}

	return reg, nil
}

// OpenAndRefreshRegistry opens and refreshes the registry.
func OpenAndRefreshRegistry(ctx context.Context, globals *GlobalOptions) (registry.CacheInterface, error) {
	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return nil, err
	}

	logger.Log(ctx).Info().Msg("Refreshing registry")
	if err := reg.Refresh(ctx); err != nil {
		return nil, fmt.Errorf("refresh registry: %w", err)
	}

	return reg, nil
}

// logProjectError logs an error with project context.
func logProjectError(ctx context.Context, err error, project registry.ProjectPath, operation string) {
	logger.Log(ctx).Warn().Err(err).Str("project", string(project)).Msg(operation)
}

// logProjectFileError logs an error with project and file context.
func logProjectFileError(ctx context.Context, project registry.ProjectPath, filePath, msg string) {
	logger.Log(ctx).Error().
		Str("project", string(project)).
		Str("file", filePath).
		Msg(msg)
}

// OpenRegistryWithRefresh opens the registry and optionally refreshes it.
func OpenRegistryWithRefresh(ctx context.Context, globals *GlobalOptions, offline bool) (registry.CacheInterface, error) {
	reg, err := OpenRegistry(ctx, globals)
	if err != nil {
		return nil, err
	}

	if !offline {
		if err := reg.Refresh(ctx); err != nil {
			logger.Log(ctx).Warn().Err(err).Msg("Failed to refresh registry")
		}
	}

	return reg, nil
}
