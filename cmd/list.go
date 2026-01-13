package cmd

import (
	"context"
	"fmt"
	"sort"

	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// ListCmd lists available projects.
type ListCmd struct {
	Local   bool `help:"List local projects instead of registry" short:"l"`
	Offline bool `help:"Don't refresh registry"`
}

// Run executes the list command.
func (c *ListCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	if c.Local {
		return c.listLocal(ctx)
	}
	return c.listRegistry(ctx, globals)
}

// listLocal lists projects in the local workspace.
func (c *ListCmd) listLocal(ctx context.Context) error {
	wctx, err := OpenWorkspaceContext(ctx)
	if err != nil {
		return err
	}

	owned, err := wctx.WS.OwnedProjects()
	if err != nil {
		return fmt.Errorf("get owned projects: %w", err)
	}

	received, err := wctx.WS.ReceivedProjects(ctx)
	if err != nil {
		return fmt.Errorf("get received projects: %w", err)
	}

	c.printLocalProjects(owned, received)
	return nil
}

// printLocalProjects prints owned and received projects.
func (c *ListCmd) printLocalProjects(owned []local.ProjectPath, received []*local.ReceivedProject) {
	if len(owned) > 0 {
		fmt.Println("Owned projects:")
		for _, p := range owned {
			fmt.Printf("  %s\n", p)
		}
	}

	if len(received) > 0 {
		fmt.Println("Pulled projects:")
		for _, r := range received {
			fmt.Printf("  %s (snapshot: %s)\n", r.Project, r.ProviderSnapshot[:7])
		}
	}

	if len(owned) == 0 && len(received) == 0 {
		fmt.Println("No projects found")
	}
}

// listRegistry lists projects from the remote registry.
func (c *ListCmd) listRegistry(ctx context.Context, globals *GlobalOptions) error {
	reg, err := OpenRegistryWithRefresh(ctx, globals, c.Offline)
	if err != nil {
		return err
	}

	return c.printRegistryProjects(ctx, reg)
}

// printRegistryProjects lists and prints all projects from the registry.
func (c *ListCmd) printRegistryProjects(ctx context.Context, reg *registry.Cache) error {
	projects, err := reg.ListProjects(ctx, nil)
	if err != nil {
		return fmt.Errorf("list projects: %w", err)
	}

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
