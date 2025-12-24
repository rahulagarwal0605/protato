package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"time"

	"github.com/alecthomas/kong"
	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/cmd"
)

// Build-time configuration
var _defaultRegistryURL = ""

// Version information (set at build time)
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

type mainCmd struct {
	cmd.GlobalOptions

	Version   versionFlag `name:"version" help:"Print version information"`
	Verbosity int         `short:"v" type:"counter" help:"Increase verbosity"`
	Dir       string      `short:"C" help:"Change directory before running"`

	Init       cmd.InitCmd   `cmd:"" help:"Initialize protato in a repository"`
	New        cmd.NewCmd    `cmd:"" help:"Create a new project (claim ownership)"`
	Pull       cmd.PullCmd   `cmd:"" help:"Download projects from registry"`
	Push       cmd.PushCmd   `cmd:"" help:"Publish owned projects to registry"`
	Verify     cmd.VerifyCmd `cmd:"" help:"Verify workspace integrity"`
	List       cmd.ListCmd   `cmd:"" help:"List available projects"`
	Mine       cmd.MineCmd   `cmd:"" help:"List files owned by this repository"`
	Completion completionCmd `cmd:"" help:"Generate shell completion scripts"`
}

type versionFlag bool

func (v versionFlag) BeforeApply(app *kong.Kong) error {
	app.Stdout.Write([]byte("protato " + version + " (" + commit + ") built on " + date + "\n"))
	os.Exit(0)
	return nil
}

type completionCmd struct {
	Shell string `arg:"" enum:"bash,zsh,fish" help:"Shell to generate completion for"`
}

func (c *completionCmd) Run(kctx *kong.Context) error {
	// Generate completions based on shell
	return nil
}

func main() {
	// Setup logging
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    !isatty.IsTerminal(os.Stderr.Fd()),
		TimeFormat: time.RFC3339,
	}
	log := zerolog.New(output).With().Timestamp().Logger()

	// Context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Signal handling
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		<-sigc
		log.Warn().Msg("Interrupted, finishing up...")
		cancel()
	}()

	// Determine default registry URL
	defaultRegistryURL := _defaultRegistryURL
	if url := os.Getenv("PROTATO_REGISTRY_URL"); url != "" {
		defaultRegistryURL = url
	}

	// Determine default cache directory
	defaultCacheDir := os.Getenv("PROTATO_REGISTRY_CACHE")
	if defaultCacheDir == "" {
		cacheHome := os.Getenv("XDG_CACHE_HOME")
		if cacheHome == "" {
			home, _ := os.UserHomeDir()
			cacheHome = filepath.Join(home, ".cache")
		}
		defaultCacheDir = filepath.Join(cacheHome, "protato", "registry")
	}

	// Parse CLI
	var cli mainCmd
	parser := kong.Must(&cli,
		kong.Name("protato"),
		kong.Description("A CLI tool for managing protobuf definitions across distributed Git repositories"),
		kong.UsageOnError(),
		kong.Vars{
			"defaultRegistryURL": defaultRegistryURL,
			"defaultCacheDir":    defaultCacheDir,
		},
		kong.BindTo(ctx, (*context.Context)(nil)),
		kong.BindTo(&log, (*zerolog.Logger)(nil)),
	)

	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	// Set verbosity
	switch cli.Verbosity {
	case 0:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	// Change directory if requested
	if cli.Dir != "" {
		if err := os.Chdir(cli.Dir); err != nil {
			log.Fatal().Err(err).Str("dir", cli.Dir).Msg("Failed to change directory")
		}
	}

	// Set default values
	if cli.RegistryURL == "" {
		cli.RegistryURL = defaultRegistryURL
	}
	if cli.CacheDir == "" {
		cli.CacheDir = defaultCacheDir
	}

	// Run command
	err = kctx.Run(&cli.GlobalOptions, &log, ctx)
	kctx.FatalIfErrorf(err)
}
