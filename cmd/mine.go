package cmd

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"

	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

// MineCmd lists files owned by this repository.
type MineCmd struct {
	Projects bool `help:"List project paths only" short:"p"`
	Absolute bool `help:"Print absolute paths" short:"a"`
}

// Run executes the mine command.
func (c *MineCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	wctx, err := OpenWorkspace(ctx)
	if err != nil {
		return err
	}

	projects, err := wctx.WS.OwnedProjects()
	if err != nil {
		return fmt.Errorf("get owned projects: %w", err)
	}

	if c.Projects {
		for _, p := range projects {
			fmt.Println(p)
		}
		return nil
	}

	return c.printFiles(ctx, wctx, projects)
}

// printFiles lists and prints all files from owned projects.
func (c *MineCmd) printFiles(ctx context.Context, wctx *WorkspaceContext, projects []local.ProjectPath) error {
	var allFiles []string

	for _, project := range projects {
		files, err := wctx.WS.ListOwnedProjectFiles(project)
		if err != nil {
			logger.Log(ctx).Warn().Err(err).Str("project", string(project)).Msg("Failed to list files")
			continue
		}

		for _, f := range files {
			path := c.formatPath(f.AbsolutePath, wctx.Repo.Root())
			allFiles = append(allFiles, path)
		}
	}

	sort.Strings(allFiles)
	for _, f := range allFiles {
		fmt.Println(f)
	}

	return nil
}

// formatPath formats the file path based on the Absolute flag.
func (c *MineCmd) formatPath(absPath, repoRoot string) string {
	if c.Absolute {
		return absPath
	}

	relPath, err := filepath.Rel(repoRoot, absPath)
	if err != nil {
		return absPath
	}
	return relPath
}
