package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// InitCmd initializes protato in a repository.
type InitCmd struct {
	Force    bool     `help:"Force overwrite existing configuration"`
	Projects []string `help:"Initial projects to claim ownership of" short:"p"`
}

// Run executes the init command.
func (c *InitCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	// Get current directory
	cwd, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get cwd: %w", err)
	}

	// Open Git repository to find root
	repo, err := git.Open(ctx, cwd, git.OpenOptions{}, log)
	if err != nil {
		return fmt.Errorf("open git repo: %w", err)
	}

	root := repo.Root()
	log.Info().Str("root", root).Msg("Initializing protato workspace")

	// Initialize workspace
	ws, err := local.Init(root, local.InitOptions{
		Force:    c.Force,
		Projects: c.Projects,
	}, log)
	if err != nil {
		return fmt.Errorf("init workspace: %w", err)
	}

	log.Info().Str("path", ws.ProtosDir()).Msg("Created protos directory")

	// Initialize registry cache if URL is provided
	if globals.RegistryURL != "" {
		log.Info().Str("url", globals.RegistryURL).Msg("Initializing registry cache")
		_, err := registry.Open(ctx, globals.CacheDir, registry.Config{
			URL: globals.RegistryURL,
		}, log)
		if err != nil {
			log.Warn().Err(err).Msg("Failed to initialize registry cache")
		}
	}

	// Create projects if specified
	if len(c.Projects) > 0 {
		log.Info().Strs("projects", c.Projects).Msg("Creating projects")
		projects := make([]local.ProjectPath, len(c.Projects))
		for i, p := range c.Projects {
			projects[i] = local.ProjectPath(p)
		}
		if err := ws.AddOwnedProjects(projects); err != nil {
			return fmt.Errorf("add projects: %w", err)
		}
	}

	log.Info().Msg("Workspace initialized successfully")
	return nil
}
