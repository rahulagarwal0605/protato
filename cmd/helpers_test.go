package cmd

import (
"context"
"io"
"testing"

"github.com/rahulagarwal0605/protato/internal/logger"
"github.com/rs/zerolog"
)

func testContext() context.Context {
	log := zerolog.New(io.Discard)
	return logger.WithLogger(context.Background(), &log)
}

func TestWorkspaceContext_Struct(t *testing.T) {
	// Test that WorkspaceContext can be created with nil values
	wctx := &WorkspaceContext{
		Repo: nil,
		WS:   nil,
	}

	if wctx.Repo != nil {
		t.Error("Expected Repo to be nil")
	}
	if wctx.WS != nil {
		t.Error("Expected WS to be nil")
	}
}

func TestGlobalOptions_Struct(t *testing.T) {
	globals := &GlobalOptions{
		CacheDir:    "/tmp/cache",
		RegistryURL: "https://github.com/example/registry.git",
	}

	if globals.CacheDir != "/tmp/cache" {
		t.Errorf("CacheDir = %v, want /tmp/cache", globals.CacheDir)
	}
	if globals.RegistryURL != "https://github.com/example/registry.git" {
		t.Errorf("RegistryURL = %v, want https://github.com/example/registry.git", globals.RegistryURL)
	}
}

func TestOpenRegistry_EmptyURL(t *testing.T) {
	ctx := testContext()
	globals := &GlobalOptions{
		CacheDir:    "/tmp/cache",
		RegistryURL: "",
	}

	_, err := OpenRegistry(ctx, globals)
	if err == nil {
		t.Error("OpenRegistry() expected error for empty URL")
	}
}
