package cmd

import (
	"testing"

	"github.com/rahulagarwal0605/protato/internal/registry"
)

func TestPullCmd_Struct(t *testing.T) {
	cmd := &PullCmd{
		Projects: []string{"team/service1"},
		Force:    true,
		NoDeps:   true,
	}

	if len(cmd.Projects) != 1 {
		t.Errorf("Projects length = %v, want 1", len(cmd.Projects))
	}
	if !cmd.Force {
		t.Error("Force should be true")
	}
	if !cmd.NoDeps {
		t.Error("NoDeps should be true")
	}
}

func TestPullCtx_Struct(t *testing.T) {
	pctx := &pullCtx{
		project:  registry.ProjectPath("team/service"),
		files:    []registry.ProjectFile{{Path: "v1/api.proto"}},
		toDelete: []string{"old_file"},
	}

	if pctx.project != "team/service" {
		t.Errorf("project = %v, want team/service", pctx.project)
	}
	if len(pctx.files) != 1 {
		t.Errorf("files length = %v, want 1", len(pctx.files))
	}
	if len(pctx.toDelete) != 1 {
		t.Errorf("toDelete length = %v, want 1", len(pctx.toDelete))
	}
}

func TestFilterOwnedProjects(t *testing.T) {
	cmd := &PullCmd{}

	tests := []struct {
		name     string
		projects []registry.ProjectPath
		owned    map[string]bool
		want     int
	}{
		{
			name:     "filter out owned",
			projects: []registry.ProjectPath{"team/service1", "team/service2", "other/service"},
			owned:    map[string]bool{"team/service1": true, "team/service2": true},
			want:     1,
		},
		{
			name:     "no owned projects",
			projects: []registry.ProjectPath{"team/service1", "team/service2"},
			owned:    map[string]bool{},
			want:     2,
		},
		{
			name:     "all owned",
			projects: []registry.ProjectPath{"team/service1"},
			owned:    map[string]bool{"team/service1": true},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := cmd.filterOwnedProjects(tt.projects, tt.owned)
			if len(got) != tt.want {
				t.Errorf("filterOwnedProjects() length = %v, want %v", len(got), tt.want)
			}
		})
	}
}
