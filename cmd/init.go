package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// InitCmd initializes protato in a repository.
type InitCmd struct {
	Force          bool     `help:"Force overwrite existing configuration"`
	Includes       []string `help:"Include patterns (glob, e.g., 'payments/**', 'orders/v*')" short:"i"`
	Excludes       []string `help:"Exclude patterns (glob, e.g., '**/test/**', 'deprecated/*')" short:"e"`
	Service        string   `help:"Service name for registry namespacing" short:"s"`
	OwnedDir       string   `help:"Directory for owned protos"`
	VendorDir      string   `help:"Directory for consumed protos"`
	SkipPrompts    bool     `help:"Skip interactive prompts and use defaults" short:"y"`
	NoAutoDiscover bool     `help:"Disable auto-discovery of projects"`
}

// Run executes the init command.
func (c *InitCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	// Find Git repository root
	root, err := c.findRepoRoot(ctx)
	if err != nil {
		return err
	}

	// Gather configuration (interactive or from flags)
	cfg := c.gatherConfig(root)

	logger.Log(ctx).Info().
		Str("root", root).
		Str("service", cfg.Service).
		Bool("auto_discover", cfg.AutoDiscover).
		Msg("Initializing protato workspace")

	// Initialize workspace
	ws, err := c.initWorkspace(ctx, root, cfg)
	if err != nil {
		return err
	}

	// Print success messages
	c.printSuccess(ws, cfg)

	// Initialize registry cache if URL is provided
	c.initRegistryCache(ctx, globals)

	// Create explicit projects if specified
	if err := c.createProjects(ctx, ws, cfg); err != nil {
		return err
	}

	// Print next steps
	c.printNextSteps(ws, cfg)

	logger.Log(ctx).Info().Msg("Workspace initialized successfully")
	return nil
}

// findRepoRoot finds the Git repository root directory.
func (c *InitCmd) findRepoRoot(ctx context.Context) (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get cwd: %w", err)
	}

	repo, err := git.Open(ctx, cwd, git.OpenOptions{})
	if err != nil {
		return "", fmt.Errorf("open git repo: %w", err)
	}

	return repo.Root(), nil
}

// gatherConfig collects configuration from flags or interactive prompts.
func (c *InitCmd) gatherConfig(root string) *local.Config {
	cfg := &local.Config{
		Service: c.Service,
		Directories: local.DirectoryConfig{
			Owned:  c.OwnedDir,
			Vendor: c.VendorDir,
		},
		// Auto-discover is enabled by default unless explicitly disabled
		AutoDiscover: !c.NoAutoDiscover,
		Includes:     c.Includes,
		Excludes:     c.Excludes,
	}

	// Use interactive mode if not skipped
	if !c.SkipPrompts {
		c.runInteractiveSetup(root, cfg)
	} else {
		// Non-interactive: apply defaults for missing values
		if cfg.Service == "" {
			cfg.Service = filepath.Base(root)
		}
		if cfg.Directories.Owned == "" {
			cfg.Directories.Owned = local.DefaultDirectoryConfig().Owned
		}
		if cfg.Directories.Vendor == "" {
			cfg.Directories.Vendor = local.DefaultDirectoryConfig().Vendor
		}
	}

	return cfg
}

// runInteractiveSetup prompts the user for configuration.
// It only prompts for fields that weren't provided via flags.
func (c *InitCmd) runInteractiveSetup(root string, cfg *local.Config) {
	reader := bufio.NewReader(os.Stdin)

	fmt.Println()
	fmt.Println("🥔 Protato Setup")
	fmt.Println()

	// Define prompt handlers - all have consistent signature (root, reader, cfg)
	prompts := []func(string, *bufio.Reader, *local.Config){
		c.promptOrShowService,
		c.promptOrShowOwnedDir,
		c.promptOrShowVendorDir,
		c.promptOrShowAutoDiscover,
		c.promptOrShowIncludes,
		c.promptOrShowExcludes,
	}

	// Execute all prompts in order
	for _, prompt := range prompts {
		prompt(root, reader, cfg)
	}

	fmt.Println()
}

// promptOrShowService prompts for service name or shows the flag value.
func (c *InitCmd) promptOrShowService(root string, reader *bufio.Reader, cfg *local.Config) {
	if c.Service == "" {
		defaultService := filepath.Base(root)
		fmt.Printf("Service name (used for registry namespace):\n  [default: %s]\n  > ", defaultService)

		input := readLine(reader)
		if input == "" {
			cfg.Service = defaultService
		} else {
			cfg.Service = input
		}
	} else {
		fmt.Printf("Service name: %s (from flag)\n", cfg.Service)
	}
}

// promptOrShowOwnedDir prompts for owned directory or shows the flag value.
func (c *InitCmd) promptOrShowOwnedDir(root string, reader *bufio.Reader, cfg *local.Config) {
	if c.OwnedDir == "" {
		defaultDir := local.DefaultDirectoryConfig().Owned
		fmt.Printf("\nDirectory for YOUR protos (protos you produce):\n  [default: %s]\n  > ", defaultDir)

		input := readLine(reader)
		if input != "" {
			cfg.Directories.Owned = input
		} else {
			cfg.Directories.Owned = defaultDir
		}
	} else {
		fmt.Printf("Owned directory: %s (from flag)\n", cfg.Directories.Owned)
	}
}

// promptOrShowVendorDir prompts for vendor directory or shows the flag value.
func (c *InitCmd) promptOrShowVendorDir(root string, reader *bufio.Reader, cfg *local.Config) {
	if c.VendorDir == "" {
		defaultDir := local.DefaultDirectoryConfig().Vendor
		fmt.Printf("\nDirectory for VENDOR protos (protos you consume):\n  [default: %s]\n  > ", defaultDir)

		input := readLine(reader)
		if input != "" {
			cfg.Directories.Vendor = input
		} else {
			cfg.Directories.Vendor = defaultDir
		}
	} else {
		fmt.Printf("Vendor directory: %s (from flag)\n", cfg.Directories.Vendor)
	}
}

// promptOrShowAutoDiscover prompts for auto-discover or shows the flag value.
func (c *InitCmd) promptOrShowAutoDiscover(root string, reader *bufio.Reader, cfg *local.Config) {
	if !c.NoAutoDiscover {
		fmt.Printf("\nAuto-discover projects? (scans for all .proto files automatically)\n  [Y/n]: ")

		input := strings.ToLower(readLine(reader))
		cfg.AutoDiscover = input != "n" && input != "no"
	} else {
		fmt.Printf("Auto-discover: %v (from flags)\n", cfg.AutoDiscover)
	}
}

// promptOrShowIncludes prompts for includes or shows the flag value.
func (c *InitCmd) promptOrShowIncludes(root string, reader *bufio.Reader, cfg *local.Config) {
	if len(c.Includes) == 0 {
		// Only prompt for includes when auto-discover is disabled
		if !cfg.AutoDiscover {
			fmt.Printf("\nInclude patterns (glob, e.g., 'payments/**', 'orders/v*'):\n  [required when auto-discover is disabled]\n  > ")

			input := readLine(reader)
			if input != "" {
				var includes []string
				for _, p := range strings.Split(input, ",") {
					p = strings.TrimSpace(p)
					if p != "" {
						includes = append(includes, p)
					}
				}
				cfg.Includes = append(cfg.Includes, includes...)
			}
		}
	} else {
		fmt.Printf("Includes: %v (from flags)\n", cfg.Includes)
	}
}

// promptOrShowExcludes prompts for excludes or shows the flag value.
func (c *InitCmd) promptOrShowExcludes(root string, reader *bufio.Reader, cfg *local.Config) {
	if len(c.Excludes) == 0 {
		fmt.Printf("\nExclude patterns (glob, e.g., '**/test/**', 'deprecated/*'):\n  [optional, press Enter to skip]\n  > ")

		input := readLine(reader)
		if input != "" {
			var excludes []string
			for _, p := range strings.Split(input, ",") {
				p = strings.TrimSpace(p)
				if p != "" {
					excludes = append(excludes, p)
				}
			}
			cfg.Excludes = append(cfg.Excludes, excludes...)
		}
	} else {
		fmt.Printf("Excludes: %v (from flags)\n", cfg.Excludes)
	}
}

// initWorkspace creates the protato workspace.
func (c *InitCmd) initWorkspace(ctx context.Context, root string, cfg *local.Config) (*local.Workspace, error) {
	ws, err := local.Init(ctx, root, cfg, c.Force)
	if err != nil {
		return nil, fmt.Errorf("init workspace: %w", err)
	}
	return ws, nil
}

// printSuccess prints success messages after initialization.
func (c *InitCmd) printSuccess(ws *local.Workspace, cfg *local.Config) {
	fmt.Printf("✅ Created protato.yaml\n")
	fmt.Printf("✅ Created %s/ directory (for your protos)\n", ws.OwnedDir())
	fmt.Printf("✅ Created %s/ directory (for vendor protos)\n", ws.VendorDir())

	if cfg.AutoDiscover {
		fmt.Printf("✅ Auto-discovery enabled (all protos in %s/ will be discovered)\n", ws.OwnedDir())
	}
}

// initRegistryCache initializes the registry cache if configured.
func (c *InitCmd) initRegistryCache(ctx context.Context, globals *GlobalOptions) {
	if globals.RegistryURL == "" {
		return
	}

	logger.Log(ctx).Info().Str("url", globals.RegistryURL).Msg("Initializing registry cache")

	_, err := registry.Open(ctx, globals.CacheDir, registry.Config{
		URL: globals.RegistryURL,
	})
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to initialize registry cache")
	}
}

// createProjects creates project directories for explicit includes when not using auto-discover.
func (c *InitCmd) createProjects(ctx context.Context, ws *local.Workspace, cfg *local.Config) error {
	// When auto-discover is enabled, projects are discovered automatically
	if cfg.AutoDiscover {
		return nil
	}

	// When auto-discover is disabled, create directories for literal paths (not glob patterns)
	literalPaths := c.extractLiteralPaths(cfg.Includes)
	if len(literalPaths) == 0 {
		return nil
	}

	logger.Log(ctx).Info().Strs("projects", literalPaths).Msg("Creating project directories")

	projects := make([]local.ProjectPath, len(literalPaths))
	for i, p := range literalPaths {
		projects[i] = local.ProjectPath(p)
	}

	if err := ws.AddOwnedProjects(projects); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	return nil
}

// extractLiteralPaths filters includes to return only literal paths (no glob patterns).
func (c *InitCmd) extractLiteralPaths(includes []string) []string {
	var literalPaths []string
	for _, pattern := range includes {
		// Check if pattern contains glob characters
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
			literalPaths = append(literalPaths, pattern)
		}
	}
	return literalPaths
}

// printNextSteps prints guidance for next steps.
func (c *InitCmd) printNextSteps(ws *local.Workspace, cfg *local.Config) {
	fmt.Println()
	fmt.Println("Next steps:")

	if cfg.AutoDiscover {
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
