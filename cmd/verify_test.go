package cmd

import (
	"testing"
)

func TestVerifyCmd_Struct(t *testing.T) {
	// Test VerifyCmd struct initialization
	cmd := &VerifyCmd{Offline: true}
	if !cmd.Offline {
		t.Error("Expected Offline to be true")
	}

	cmd2 := &VerifyCmd{Offline: false}
	if cmd2.Offline {
		t.Error("Expected Offline to be false")
	}
}

func TestVerifyCtx_Struct(t *testing.T) {
	// Test verifyCtx can be created
	vctx := &verifyCtx{
		wctx:    nil,
		reg:     nil,
		repoURL: "https://github.com/test/repo",
	}

	if vctx.repoURL != "https://github.com/test/repo" {
		t.Errorf("Expected repoURL to be 'https://github.com/test/repo', got %q", vctx.repoURL)
	}
}
