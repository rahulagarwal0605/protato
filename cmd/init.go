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
	Force     bool     `help:"Force overwrite existing configuration"`
	Projects  []string `help:"Initial projects to claim ownership of" short:"p"`
	Service   string   `help:"Service name for registry namespacing" short:"s"`
	OwnedDir  string   `help:"Directory for owned protos" name:"owned-dir" default:""`
	VendorDir string   `help:"Directory for consumed protos" name:"vendor-dir" default:""`
	Yes       bool     `help:"Skip interactive prompts and use defaults" short:"y"`
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

	// Get service name (from flag, repo name, or prompt)
	serviceName := c.Service
	ownedDir := c.OwnedDir
	vendorDir := c.VendorDir

	if !c.Yes && serviceName == "" {
		// Interactive mode
		reader := bufio.NewReader(os.Stdin)

		fmt.Println()
		fmt.Println("🥔 Protato Setup")
		fmt.Println()

		// Service name
		defaultService := filepath.Base(root)
		fmt.Printf("Service name (used for registry namespace):\n")
		fmt.Printf("  [default: %s]\n  > ", defaultService)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input == "" {
			serviceName = defaultService
		} else {
			serviceName = input
		}

		// Owned directory
		fmt.Printf("\nDirectory for YOUR protos (protos you produce):\n")
		fmt.Printf("  [default: proto]\n  > ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			ownedDir = input
		}

		// Vendor directory
		fmt.Printf("\nDirectory for VENDOR protos (protos you consume from other services):\n")
		fmt.Printf("  [default: vendor-proto]\n  > ")
		input, _ = reader.ReadString('\n')
		input = strings.TrimSpace(input)
		if input != "" {
			vendorDir = input
		}

		fmt.Println()
	} else if serviceName == "" {
		// Non-interactive: use repo name as default service name
		serviceName = filepath.Base(root)
	}

	log.Info().Str("root", root).Str("service", serviceName).Msg("Initializing protato workspace")

	// Initialize workspace
	ws, err := local.Init(root, local.InitOptions{
		Force:     c.Force,
		Projects:  c.Projects,
		Service:   serviceName,
		OwnedDir:  ownedDir,
		VendorDir: vendorDir,
	}, log)
	if err != nil {
		return fmt.Errorf("init workspace: %w", err)
	}

	fmt.Printf("✅ Created protato.yaml\n")
	fmt.Printf("✅ Created %s/ directory (for your protos)\n", ws.OwnedDir())
	fmt.Printf("✅ Created %s/ directory (for vendor protos)\n", ws.VendorDir())

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

	fmt.Println()
	fmt.Println("Next steps:")
	fmt.Printf("  1. Add your proto projects: protato new <project-path>\n")
	fmt.Printf("  2. Push to registry: protato push\n")
	fmt.Printf("  3. Pull dependencies: protato pull <project-path>\n")
	fmt.Println()

	log.Info().Msg("Workspace initialized successfully")
	return nil
}
