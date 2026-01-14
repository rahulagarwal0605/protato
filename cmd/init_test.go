package cmd

import (
	"testing"

	"github.com/rahulagarwal0605/protato/internal/local"
)

func TestInitCmd_Struct(t *testing.T) {
	cmd := &InitCmd{
		SkipPrompts:    true,
		Service:        "my-service",
		OwnedDir:       "proto",
		VendorDir:      "vendor",
		NoAutoDiscover: false,
	}

	if !cmd.SkipPrompts {
		t.Error("SkipPrompts should be true")
	}
	if cmd.Service != "my-service" {
		t.Errorf("Service = %v, want my-service", cmd.Service)
	}
	if cmd.OwnedDir != "proto" {
		t.Errorf("OwnedDir = %v, want proto", cmd.OwnedDir)
	}
	if cmd.VendorDir != "vendor" {
		t.Errorf("VendorDir = %v, want vendor", cmd.VendorDir)
	}
}

func TestValidateConfig(t *testing.T) {
	cmd := &InitCmd{}

	tests := []struct {
		name    string
		cfg     *local.Config
		wantErr bool
	}{
		{
			name: "valid config without auto-discover",
			cfg: &local.Config{
				Service:      "my-service",
				AutoDiscover: false,
				Projects:     []string{"team/service1"},
			},
			wantErr: false,
		},
		{
			name: "valid config with auto-discover no projects",
			cfg: &local.Config{
				Service:      "my-service",
				AutoDiscover: true,
				Projects:     []string{},
			},
			wantErr: false,
		},
		{
			name: "invalid config - auto-discover with projects",
			cfg: &local.Config{
				Service:      "my-service",
				AutoDiscover: true,
				Projects:     []string{"team/service1"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
err := cmd.validateConfig(tt.cfg)
if (err != nil) != tt.wantErr {
t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
}
})
	}
}
