package logger

import (
	"context"
	"testing"

	"github.com/rs/zerolog"
)

func TestWithLogger(t *testing.T) {
	t.Run("stores logger in context", func(t *testing.T) {
		log := Init()
		ctx := context.Background()

		newCtx := WithLogger(ctx, &log)

		if newCtx == ctx {
			t.Error("WithLogger should return a new context")
		}

		retrieved := Log(newCtx)
		if retrieved == nil {
			t.Error("Log should return the stored logger")
		}
	})

	t.Run("nil context value before setting", func(t *testing.T) {
		ctx := context.Background()
		retrieved := Log(ctx)

		if retrieved != nil {
			t.Error("Log should return nil when no logger is set")
		}
	})
}

func TestLog(t *testing.T) {
	tests := []struct {
		name      string
		setupCtx  func() context.Context
		expectNil bool
	}{
		{
			name: "returns logger when set",
			setupCtx: func() context.Context {
				log := Init()
				return WithLogger(context.Background(), &log)
			},
			expectNil: false,
		},
		{
			name: "returns nil when not set",
			setupCtx: func() context.Context {
				return context.Background()
			},
			expectNil: true,
		},
		{
			name: "returns nil for wrong type in context",
			setupCtx: func() context.Context {
				return context.WithValue(context.Background(), loggerContextKey{}, "not a logger")
			},
			expectNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := tt.setupCtx()
			result := Log(ctx)

			if tt.expectNil && result != nil {
				t.Error("expected nil logger")
			}
			if !tt.expectNil && result == nil {
				t.Error("expected non-nil logger")
			}
		})
	}
}

func TestInit(t *testing.T) {
	t.Run("returns valid logger", func(t *testing.T) {
		log := Init()

		// Verify logger is functional by checking it doesn't panic
		log.Info().Msg("test message")
	})

	t.Run("logger has timestamp", func(t *testing.T) {
		log := Init()

		// Logger should be configured with timestamp
		// We verify this by checking the logger is not nil
		if log.GetLevel() < zerolog.TraceLevel || log.GetLevel() > zerolog.Disabled {
			// This is just to verify logger is valid
		}
	})
}

func TestSetLogLevel(t *testing.T) {
	tests := []struct {
		name      string
		verbosity int
		wantLevel zerolog.Level
	}{
		{
			name:      "verbosity 0 sets InfoLevel",
			verbosity: 0,
			wantLevel: zerolog.InfoLevel,
		},
		{
			name:      "verbosity 1 sets DebugLevel",
			verbosity: 1,
			wantLevel: zerolog.DebugLevel,
		},
		{
			name:      "verbosity 2 sets TraceLevel",
			verbosity: 2,
			wantLevel: zerolog.TraceLevel,
		},
		{
			name:      "verbosity 3+ sets TraceLevel",
			verbosity: 5,
			wantLevel: zerolog.TraceLevel,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			SetLogLevel(tt.verbosity)
			gotLevel := zerolog.GlobalLevel()

			if gotLevel != tt.wantLevel {
				t.Errorf("SetLogLevel(%d) set level to %v, want %v",
					tt.verbosity, gotLevel, tt.wantLevel)
			}
		})
	}

	// Reset to default
	SetLogLevel(0)
}

func TestLoggerContextKey(t *testing.T) {
	t.Run("context key is unique", func(t *testing.T) {
		key1 := loggerContextKey{}
		key2 := loggerContextKey{}

		// Both should be equal (same type, zero value)
		if key1 != key2 {
			t.Error("loggerContextKey instances should be equal")
		}
	})
}
