package cmd

import (
	"context"
	"fmt"
	"os"
	"sort"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// ListCmd lists available projects.
type ListCmd struct {
	Local   bool `help:"List local projects instead of registry" short:"l"`
	Offline bool `help:"Don't refresh registry" name:"offline"`
}

// Run executes the list command.
func (c *ListCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	if c.Local {
		return c.listLocal(globals, log, ctx)
	}
	return c.listRegistry(globals, log, ctx)
}

func (c *ListCmd) listLocal(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
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
	owned, err := ws.OwnedProjects()
	if err != nil {
		return fmt.Errorf("get owned projects: %w", err)
	}

	// Get received projects
	received, err := ws.ReceivedProjects()
	if err != nil {
		return fmt.Errorf("get received projects: %w", err)
	}

	// Print owned projects
	if len(owned) > 0 {
		fmt.Println("Owned projects:")
		for _, p := range owned {
			fmt.Printf("  %s\n", p)
		}
	}

	// Print received projects
	if len(received) > 0 {
		fmt.Println("Pulled projects:")
		for _, r := range received {
			fmt.Printf("  %s (snapshot: %s)\n", r.Project, r.ProviderSnapshot[:7])
		}
	}

	if len(owned) == 0 && len(received) == 0 {
		fmt.Println("No projects found")
	}

	return nil
}

func (c *ListCmd) listRegistry(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	if globals.RegistryURL == "" {
		return fmt.Errorf("registry URL not configured")
	}

	// Open registry
	reg, err := registry.Open(ctx, globals.CacheDir, registry.Config{
		URL: globals.RegistryURL,
	}, log)
	if err != nil {
		return fmt.Errorf("open registry: %w", err)
	}

	// Refresh unless offline
	if !c.Offline {
		log.Debug().Msg("Refreshing registry")
		if err := reg.Refresh(ctx); err != nil {
			log.Warn().Err(err).Msg("Failed to refresh registry")
		}
	}

	// List projects
	projects, err := reg.ListProjects(ctx, nil)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

	// Sort and print
	projectStrings := make([]string, len(projects))
	for i, p := range projects {
		projectStrings[i] = string(p)
	}
	sort.Strings(projectStrings)

	for _, p := range projectStrings {
		fmt.Println(p)
	}

	if len(projects) == 0 {
		fmt.Println("No projects in registry")
	}

	return nil
}
