package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// InitCmd initializes protato in a repository.
type InitCmd struct {
	Force          bool     `help:"Force overwrite existing configuration"`
	Projects       []string `help:"Initial projects to claim ownership of (disables auto-discover)" short:"p"`
	Service        string   `help:"Service name for registry namespacing" short:"s"`
	OwnedDir       string   `help:"Directory for owned protos" name:"owned-dir" default:""`
	VendorDir      string   `help:"Directory for consumed protos" name:"vendor-dir" default:""`
	Yes            bool     `help:"Skip interactive prompts and use defaults" short:"y"`
	NoAutoDiscover bool     `help:"Disable auto-discovery of projects" name:"no-auto-discover"`
}

// initConfig holds the configuration gathered during init.
type initConfig struct {
	serviceName  string
	ownedDir     string
	vendorDir    string
	autoDiscover bool
	projects     []string
}

// Run executes the init command.
func (c *InitCmd) Run(globals *GlobalOptions, log *zerolog.Logger, ctx context.Context) error {
	// Find Git repository root
	root, err := c.findRepoRoot(ctx, log)
	if err != nil {
		return err
	}

	// Gather configuration (interactive or from flags)
	cfg := c.gatherConfig(root)

	log.Info().
		Str("root", root).
		Str("service", cfg.serviceName).
		Bool("auto_discover", cfg.autoDiscover).
		Msg("Initializing protato workspace")

	// Initialize workspace
	ws, err := c.initWorkspace(root, cfg, log)
	if err != nil {
		return err
	}

	// Print success messages
	c.printSuccess(ws, cfg)

	// Initialize registry cache if URL is provided
	c.initRegistryCache(ctx, globals, log)

	// Create explicit projects if specified
	if err := c.createProjects(ws, cfg, log); err != nil {
		return err
	}

	// Print next steps
	c.printNextSteps(ws, cfg)

	log.Info().Msg("Workspace initialized successfully")
	return nil
}

// findRepoRoot finds the Git repository root directory.
func (c *InitCmd) findRepoRoot(ctx context.Context, log *zerolog.Logger) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	repo, err := git.Open(ctx, cwd, git.OpenOptions{}, log)
	if err != nil {
		return "", fmt.Errorf("open git repo: %w", err)
	}

	return repo.Root(), nil
}

// gatherConfig collects configuration from flags or interactive prompts.
func (c *InitCmd) gatherConfig(root string) initConfig {
	cfg := initConfig{
		serviceName:  c.Service,
		ownedDir:     c.OwnedDir,
		vendorDir:    c.VendorDir,
		autoDiscover: !c.NoAutoDiscover && len(c.Projects) == 0,
		projects:     c.Projects,
	}

	// Use interactive mode if not skipped and service name not provided
	if !c.Yes && c.Service == "" {
		c.runInteractiveSetup(root, &cfg)
	} else if cfg.serviceName == "" {
		// Non-interactive: use repo name as default
		cfg.serviceName = filepath.Base(root)
	}

	return cfg
}

// runInteractiveSetup prompts the user for configuration.
func (c *InitCmd) runInteractiveSetup(root string, cfg *initConfig) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("🥔 Protato Setup")
	fmt.Println()

	// Prompt for each configuration option
	cfg.serviceName = c.promptServiceName(reader, root)
	cfg.ownedDir = c.promptDirectory(reader, "YOUR protos (protos you produce)", "proto")
	cfg.vendorDir = c.promptDirectory(reader, "VENDOR protos (protos you consume)", "vendor-proto")
	cfg.autoDiscover, cfg.projects = c.promptAutoDiscover(reader, cfg.projects)

	fmt.Println()
}

// promptServiceName prompts for the service name.
func (c *InitCmd) promptServiceName(reader *bufio.Reader, root string) string {
	defaultService := filepath.Base(root)
	fmt.Printf("Service name (used for registry namespace):\n")
	fmt.Printf("  [default: %s]\n  > ", defaultService)

	input := readLine(reader)
	if input == "" {
		return defaultService
	}
	return input
}

// promptDirectory prompts for a directory path.
func (c *InitCmd) promptDirectory(reader *bufio.Reader, description, defaultVal string) string {
	fmt.Printf("\nDirectory for %s:\n", description)
	fmt.Printf("  [default: %s]\n  > ", defaultVal)

	input := readLine(reader)
	if input != "" {
		return input
	}
	return ""
}

// promptAutoDiscover prompts for auto-discover preference.
func (c *InitCmd) promptAutoDiscover(reader *bufio.Reader, existingProjects []string) (bool, []string) {
	fmt.Printf("\nAuto-discover projects? (scans for all .proto files automatically)\n")
	fmt.Printf("  [Y/n]: ")

	input := strings.ToLower(readLine(reader))

	if input == "n" || input == "no" {
		projects := c.promptProjects(reader)
		return false, append(existingProjects, projects...)
	}

	return true, existingProjects
}

// promptProjects prompts for project paths when auto-discover is disabled.
func (c *InitCmd) promptProjects(reader *bufio.Reader) []string {
	fmt.Printf("\nEnter project paths (comma-separated, e.g., api/v1,events/v1):\n")
	fmt.Printf("  > ")

	input := readLine(reader)
	if input == "" {
		return nil
	}

	var projects []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			projects = append(projects, p)
		}
	}
	return projects
}

// initWorkspace creates the protato workspace.
func (c *InitCmd) initWorkspace(root string, cfg initConfig, log *zerolog.Logger) (*local.Workspace, error) {
	ws, err := local.Init(root, local.InitOptions{
		Force:        c.Force,
		Projects:     cfg.projects,
		Service:      cfg.serviceName,
		OwnedDir:     cfg.ownedDir,
		VendorDir:    cfg.vendorDir,
		AutoDiscover: cfg.autoDiscover,
	}, log)
	if err != nil {
		return nil, fmt.Errorf("init workspace: %w", err)
	}
	return ws, nil
}

// printSuccess prints success messages after initialization.
func (c *InitCmd) printSuccess(ws *local.Workspace, cfg initConfig) {
	fmt.Printf("✅ Created protato.yaml\n")
	fmt.Printf("✅ Created %s/ directory (for your protos)\n", ws.OwnedDir())
	fmt.Printf("✅ Created %s/ directory (for vendor protos)\n", ws.VendorDir())

	if cfg.autoDiscover {
		fmt.Printf("✅ Auto-discovery enabled (all protos in %s/ will be discovered)\n", ws.OwnedDir())
	}
}

// initRegistryCache initializes the registry cache if configured.
func (c *InitCmd) initRegistryCache(ctx context.Context, globals *GlobalOptions, log *zerolog.Logger) {
	if globals.RegistryURL == "" {
		return
	}

	log.Info().Str("url", globals.RegistryURL).Msg("Initializing registry cache")

	_, err := registry.Open(ctx, globals.CacheDir, registry.Config{
		URL: globals.RegistryURL,
	}, log)
	if err != nil {
		log.Warn().Err(err).Msg("Failed to initialize registry cache")
	}
}

// createProjects creates explicit projects when not using auto-discover.
func (c *InitCmd) createProjects(ws *local.Workspace, cfg initConfig, log *zerolog.Logger) error {
	if len(cfg.projects) == 0 || cfg.autoDiscover {
		return nil
	}

	log.Info().Strs("projects", cfg.projects).Msg("Creating projects")

	projects := make([]local.ProjectPath, len(cfg.projects))
	for i, p := range cfg.projects {
		projects[i] = local.ProjectPath(p)
	}

	if err := ws.AddOwnedProjects(projects); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}
	return nil
}

// printNextSteps prints guidance for next steps.
func (c *InitCmd) printNextSteps(ws *local.Workspace, cfg initConfig) {
	fmt.Println()
	fmt.Println("Next steps:")

	if cfg.autoDiscover {
		fmt.Printf("  1. Add your .proto files to %s/<project>/\n", ws.OwnedDir())
	} else {
		fmt.Printf("  1. Add your proto projects: protato new <project-path>\n")
	}

	fmt.Printf("  2. Push to registry: protato push\n")
	fmt.Printf("  3. Pull dependencies: protato pull <project-path>\n")
	fmt.Println()
}

// readLine reads a line from the reader and trims whitespace.
func readLine(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}
