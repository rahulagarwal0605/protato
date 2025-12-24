package cmd

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
)

// MineCmd lists files owned by this repository.
type MineCmd struct {
	Projects bool `help:"List project paths only" short:"p"`
	Absolute bool `help:"Print absolute paths" short:"a"`
}

// Run executes the mine command.
func (c *MineCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
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
	projects, err := ws.OwnedProjects()
	if err != nil {
		return fmt.Errorf("get owned projects: %w", err)
	}

	if c.Projects {
		// Print project paths only
		for _, p := range projects {
			fmt.Println(p)
		}
		return nil
	}

	// Print file paths
	var allFiles []string

	for _, project := range projects {
		files, err := ws.ListProjectFiles(project)
		if err != nil {
			log.Warn().Err(err).Str("project", string(project)).Msg("Failed to list files")
			continue
		}

		for _, f := range files {
			var path string
			if c.Absolute {
				path = f.AbsolutePath
			} else {
				relPath, err := filepath.Rel(repo.Root(), f.AbsolutePath)
				if err != nil {
					path = f.AbsolutePath
				} else {
					path = relPath
				}
			}
			allFiles = append(allFiles, path)
		}
	}

	// Sort and print
	sort.Strings(allFiles)
	for _, f := range allFiles {
		fmt.Println(f)
	}

	return nil
}
