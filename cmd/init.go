package cmd

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/local"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// InitCmd initializes protato in a repository.
type InitCmd struct {
	Force          bool     `help:"Force overwrite existing configuration"`
	Projects       []string `help:"Project patterns (glob) to find projects when auto-discover is disabled. Examples: 'payments/**', 'orders/v*'. Only used when auto_discover=false." short:"p"`
	Ignores        []string `help:"Ignore patterns (glob) to exclude projects or files. Examples: '**/test/**' (exclude test projects/files), 'deprecated/*' (exclude deprecated projects). Works with both auto_discover=true (filter projects) and auto_discover=false (filter files)." short:"i"`
	Service        string   `help:"Service name for registry namespacing" short:"s"`
	OwnedDir       string   `help:"Directory for owned protos"`
	VendorDir      string   `help:"Directory for consumed protos"`
	SkipPrompts    bool     `help:"Skip interactive prompts and use defaults" short:"y"`
	NoAutoDiscover bool     `help:"Disable auto-discovery of projects"`
}

// Run executes the init command.
func (c *InitCmd) Run(globals *GlobalOptions, ctx context.Context) error {
	// Get current Git repository
	repo, err := GetCurrentRepo(ctx)
	if err != nil {
		return err
	}

	// Gather configuration (interactive or from flags)
	cfg := c.gatherConfig(repo.Root())

	// Validate configuration consistency
	if err := c.validateConfig(cfg); err != nil {
		return err
	}

	logger.Log(ctx).Info().
		Str("root", repo.Root()).
		Str("service", cfg.Service).
		Bool("auto_discover", cfg.AutoDiscover).
		Msg("Initializing protato workspace")

	// Initialize workspace
	ws, err := c.initWorkspace(ctx, repo.Root(), cfg)
	if err != nil {
		return err
	}

	// Create explicit projects if specified (must happen before success messages)
	if err := c.createProjects(ctx, ws, cfg); err != nil {
		return err
	}

	// Initialize registry cache if URL is provided
	c.initRegistryCache(ctx, globals)

	// Print completion messages and next steps
	if err := c.printCompletion(ws, cfg); err != nil {
		return err
	}

	logger.Log(ctx).Info().Msg("Workspace initialized successfully")
	return nil
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
		Projects:     c.Projects,
		Ignores:      c.Ignores,
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

// validateConfig validates the configuration for consistency.
func (c *InitCmd) validateConfig(cfg *local.Config) error {
	// If auto_discover=true, projects should be empty (projects are skipped)
	if cfg.AutoDiscover && len(cfg.Projects) > 0 {
		return fmt.Errorf("projects cannot be set when auto_discover=true (projects are skipped when auto-discover is enabled)")
	}

	return nil
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
		c.promptOrShowProjects,
		c.promptOrShowIgnores,
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

// promptOrShowProjects prompts for projects or shows the flag value.
// Only prompts when auto_discover=false, as projects are used to find projects matching patterns.
func (c *InitCmd) promptOrShowProjects(root string, reader *bufio.Reader, cfg *local.Config) {
	if len(c.Projects) == 0 {
		// Only prompt for projects when auto-discover is disabled
		if !cfg.AutoDiscover {
			fmt.Printf("\nProject patterns (glob, e.g., 'payments/**', 'orders/v*'):\n  [required when auto-discover is disabled]\n  > ")

			input := readLine(reader)
			if input != "" {
				projects := ParseCommaSeparated(input)
				cfg.Projects = append(cfg.Projects, projects...)
			}
		}
	} else {
		fmt.Printf("Projects: %v (from flags)\n", cfg.Projects)
	}
}

// promptOrShowIgnores prompts for ignores or shows the flag value.
// Ignores can be used in both auto_discover=true (filter discovered projects) and auto_discover=false (filter files within projects).
func (c *InitCmd) promptOrShowIgnores(root string, reader *bufio.Reader, cfg *local.Config) {
	if len(c.Ignores) == 0 {
		fmt.Printf("\nIgnore patterns (glob, e.g., '**/test/**', 'deprecated/*'):\n  [optional, press Enter to skip]\n  > ")

		input := readLine(reader)
		if input != "" {
			ignores := ParseCommaSeparated(input)
			cfg.Ignores = append(cfg.Ignores, ignores...)
		}
	} else {
		fmt.Printf("Ignores: %v (from flags)\n", cfg.Ignores)
	}
}

// readLine reads a line from the reader and trims whitespace.
func readLine(reader *bufio.Reader) string {
	input, _ := reader.ReadString('\n')
	return strings.TrimSpace(input)
}

// initWorkspace creates the protato workspace.
func (c *InitCmd) initWorkspace(ctx context.Context, root string, cfg *local.Config) (*local.Workspace, error) {
	ws, err := local.Init(ctx, root, cfg, c.Force)
	if err != nil {
		return nil, fmt.Errorf("init workspace: %w", err)
	}
	return ws, nil
}

// createProjects creates project directories for explicit projects when not using auto-discover.
func (c *InitCmd) createProjects(ctx context.Context, ws *local.Workspace, cfg *local.Config) error {
	// When auto-discover is enabled, projects are discovered automatically
	if cfg.AutoDiscover {
		return nil
	}

	// When auto-discover is disabled, create directories for literal paths (not glob patterns)
	literalPaths := ExtractLiteralPaths(cfg.Projects)
	if len(literalPaths) == 0 {
		return nil
	}

	logger.Log(ctx).Info().Strs("projects", literalPaths).Msg("Creating project directories")

	if err := ws.AddOwnedProjects(literalPaths); err != nil {
		return fmt.Errorf("add projects: %w", err)
	}

	return nil
}

// initRegistryCache initializes the registry cache if configured.
func (c *InitCmd) initRegistryCache(ctx context.Context, globals *GlobalOptions) {
	if globals.RegistryURL == "" {
		return
	}

	logger.Log(ctx).Info().Msg("Initializing registry cache")

	_, err := registry.Open(ctx, globals.CacheDir, globals.RegistryURL)
	if err != nil {
		logger.Log(ctx).Warn().Err(err).Msg("Failed to initialize registry cache")
	}
}

// printCompletion prints success messages and next steps after initialization.
func (c *InitCmd) printCompletion(ws *local.Workspace, cfg *local.Config) error {
	ownedDir, err := ws.OwnedDir()
	if err != nil {
		return fmt.Errorf("get owned directory: %w", err)
	}
	vendorDir, err := ws.VendorDir()
	if err != nil {
		return fmt.Errorf("get vendor directory: %w", err)
	}

	fmt.Printf("✅ Created protato.yaml\n")
	fmt.Printf("✅ Created %s/ directory (for your protos)\n", ownedDir)
	fmt.Printf("✅ Created %s/ directory (for vendor protos)\n", vendorDir)

	if cfg.AutoDiscover {
		fmt.Printf("✅ Auto-discovery enabled (all protos in %s/ will be discovered)\n", ownedDir)
	}

	fmt.Println()
	fmt.Println("Next steps:")

	if cfg.AutoDiscover {
		fmt.Printf("  1. Add your .proto files to %s/<project>/\n", ownedDir)
	} else {
		fmt.Printf("  1. Add your proto projects: protato new <project-path>\n")
	}

	fmt.Printf("  2. Push to registry: protato push\n")
	fmt.Printf("  3. Pull dependencies: protato pull <project-path>\n")
	fmt.Println()

	return nil
}
