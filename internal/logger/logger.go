package logger

import (
	"context"
	"os"
	"time"

	"github.com/mattn/go-isatty"
	"github.com/rs/zerolog"
)

// loggerContextKey is the context key for the logger.
// Using a private struct type ensures no collisions with other context keys.
type loggerContextKey struct{}

// WithLogger returns a context with the given logger.
// This should be called once at the application entry point (e.g., main.go)
// to inject the logger into the context.
func WithLogger(ctx context.Context, log *zerolog.Logger) context.Context {
	return context.WithValue(ctx, loggerContextKey{}, log)
}

// Log returns the logger from context for convenient inline logging.
// Usage: Log(ctx).Info().Msg("message") or Log(ctx).Debug().Str("key", "value").Msg("debug")
// Returns nil if no logger is found in the context (zerolog handles nil gracefully).
func Log(ctx context.Context) *zerolog.Logger {
	if log, ok := ctx.Value(loggerContextKey{}).(*zerolog.Logger); ok {
		return log
	}
	return nil
}

// Init creates a configured zerolog logger with console output.
// This should be called once at application startup to create the logger instance.
func Init() zerolog.Logger {
	output := zerolog.ConsoleWriter{
		Out:        os.Stderr,
		NoColor:    !isatty.IsTerminal(os.Stderr.Fd()),
		TimeFormat: time.RFC3339,
	}
	return zerolog.New(output).With().Timestamp().Logger()
}

// SetLogLevel sets the global log level based on verbosity count.
//   - verbosity 0: InfoLevel
//   - verbosity 1: DebugLevel
//   - verbosity 2+: TraceLevel
func SetLogLevel(verbosity int) {
	switch verbosity {
	case 0:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 1:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}
}
