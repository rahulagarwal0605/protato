package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"

	"github.com/alecthomas/kong"
	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/cmd"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

// Version information (set at build time via -ldflags)
// Example: go build -ldflags "-X main.version=v1.0.0 -X main.commit=abc123 -X main.date=2024-01-01T00:00:00Z"
var (
	version string
	commit  string
	date    string
)

type mainCmd struct {
	cmd.GlobalOptions

	Version   versionFlag `name:"version" help:"Print version information"`
	Verbosity int         `short:"v" type:"counter" help:"Increase verbosity"`
	Dir       string      `short:"C" help:"Change directory before running"`

	Init   cmd.InitCmd   `cmd:"" help:"Initialize protato in a repository"`
	New    cmd.NewCmd    `cmd:"" help:"Create a new project (claim ownership)"`
	Pull   cmd.PullCmd   `cmd:"" help:"Download projects from registry"`
	Push   cmd.PushCmd   `cmd:"" help:"Publish owned projects to registry"`
	Verify cmd.VerifyCmd `cmd:"" help:"Verify workspace integrity"`
	List   cmd.ListCmd   `cmd:"" help:"List available projects"`
	Mine   cmd.MineCmd   `cmd:"" help:"List files owned by this repository"`
}

type versionFlag bool

func main() {
	ctx, cancel := setupContextAndLogging()
	defer cancel()

	defaultCacheDir, err := getDefaultCacheDir()
	if err != nil {
		logger.Log(ctx).Fatal().Err(err).Msg("Failed to determine cache directory")
	}
	cli, parser := setupCLI(ctx, defaultCacheDir)

	kctx, err := parser.Parse(os.Args[1:])
	if err != nil {
		parser.FatalIfErrorf(err)
	}

	configureVerbosity(cli.Verbosity)
	configureDirectory(ctx, cli.Dir)

	// Execute command - Kong injects globals and ctx
	if err := kctx.Run(&cli.GlobalOptions, ctx); err != nil {
		// If context was cancelled (e.g., Ctrl+C), exit cleanly without error message
		if err == context.Canceled {
			os.Exit(130) // Standard exit code for SIGINT (Ctrl+C)
		}
		kctx.FatalIfErrorf(err)
	}
}

// setupContextAndLogging creates context and logger with signal handling.
// The logger is injected into the context before returning.
func setupContextAndLogging() (context.Context, context.CancelFunc) {
	log := logger.Init()

	ctx, cancel := context.WithCancel(context.Background())
	// Inject logger into context
	ctx = logger.WithLogger(ctx, &log)
	setupSignalHandling(ctx, cancel)

	return ctx, cancel
}

// setupSignalHandling sets up interrupt signal handling.
func setupSignalHandling(ctx context.Context, cancel context.CancelFunc) {
	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt)
	go func() {
		<-sigc
		logger.Log(ctx).Warn().Msg("Interrupted, finishing up...")
		cancel()
	}()
}

// getDefaultCacheDir returns the OS-specific default cache directory.
// Note: PROTATO_REGISTRY_CACHE env var is handled by Kong's env tag in GlobalOptions.
// This function only calculates the fallback default when env var is not set.
//   - macOS: ~/Library/Caches/
//   - Windows: %LOCALAPPDATA%
//   - Linux/Unix: $XDG_CACHE_HOME or ~/.cache
func getDefaultCacheDir() (string, error) {
	// Use Go's cross-platform cache directory function
	// os.UserCacheDir() is very reliable and handles all OS-specific conventions
	cacheHome, err := os.UserCacheDir()
	if err != nil {
		// Fallback: use home directory + .cache
		userCacheErr := err
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("failed to determine cache directory: UserCacheDir failed (%v), UserHomeDir failed (%v)", userCacheErr, err)
		}
		cacheHome = filepath.Join(home, ".cache")
	}

	return filepath.Join(cacheHome, "protato", "registry"), nil
}

// setupCLI creates and configures the Kong CLI parser.
func setupCLI(ctx context.Context, defaultCacheDir string) (*mainCmd, *kong.Kong) {
	cli := &mainCmd{}

	parser := kong.Must(cli,
		kong.Name("protato"),
		kong.Description("A CLI tool for managing protobuf definitions across distributed Git repositories"),
		kong.UsageOnError(),
		kong.Vars{
			"defaultCacheDir": defaultCacheDir, // Used by Kong's default interpolation in struct tags
		},
		kong.BindTo(ctx, (*context.Context)(nil)),
	)

	return cli, parser
}

// configureVerbosity sets the global log level based on verbosity flag.
func configureVerbosity(verbosity int) {
	switch verbosity {
	case 0:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}

// configureDirectory changes to the requested directory if specified.
func configureDirectory(ctx context.Context, dir string) {
	if dir == "" {
		return
	}

	if err := os.Chdir(dir); err != nil {
		logger.Log(ctx).Fatal().Err(err).Str("dir", dir).Msg("Failed to change directory")
	}
}

// BeforeApply handles the --version flag by printing version info and exiting.
func (v versionFlag) BeforeApply(app *kong.Kong) error {
	// Format version string, handling empty values
	versionStr := version
	if versionStr == "" {
		versionStr = "unknown"
	}
	commitStr := commit
	if commitStr == "" {
		commitStr = "unknown"
	}
	dateStr := date
	if dateStr == "" {
		dateStr = "unknown"
	}

	app.Stdout.Write([]byte("protato " + versionStr + " (" + commitStr + ") built on " + dateStr + "\n"))
	os.Exit(0)
	return nil
}
