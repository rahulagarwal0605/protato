package registry

import (
	"errors"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/constants"
	"github.com/rahulagarwal0605/protato/internal/git"
)

func TestProtosPath(t *testing.T) {
	tests := []struct {
		name  string
		parts []string
		want  string
	}{
		{
			name:  "single part",
			parts: []string{"team/service"},
			want:  constants.ProtosDir + "/team/service",
		},
		{
			name:  "multiple parts",
			parts: []string{"team", "service", "v1"},
			want:  constants.ProtosDir + "/team/service/v1",
		},
		{
			name:  "no parts",
			parts: []string{},
			want:  constants.ProtosDir,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := protosPath(tt.parts...)
			if got != tt.want {
				t.Errorf("protosPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsBlobType(t *testing.T) {
	tests := []struct {
		name      string
		entryType git.ObjectType
		want      bool
	}{
		{
			name:      "blob type",
			entryType: git.BlobType,
			want:      true,
		},
		{
			name:      "tree type",
			entryType: git.TreeType,
			want:      false,
		},
		{
			name:      "commit type",
			entryType: git.CommitType,
			want:      false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isBlobType(tt.entryType)
			if got != tt.want {
				t.Errorf("isBlobType() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimProtosPrefix(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "path with protos prefix",
			path: constants.ProtosDir + "/team/service",
			want: "team/service",
		},
		{
			name: "path without protos prefix",
			path: "team/service",
			want: "team/service",
		},
		{
			name: "just protos dir",
			path: constants.ProtosDir,
			want: constants.ProtosDir,
		},
		{
			name: "empty path",
			path: "",
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimProtosPrefix(tt.path)
			if got != tt.want {
				t.Errorf("trimProtosPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRefspec(t *testing.T) {
	tests := []struct {
		name string
		src  string
		dst  string
		want git.Refspec
	}{
		{
			name: "branch refspec",
			src:  "refs/heads/main",
			dst:  "refs/remotes/origin/main",
			want: git.Refspec("refs/heads/main:refs/remotes/origin/main"),
		},
		{
			name: "tag refspec",
			src:  "refs/tags/v1.0.0",
			dst:  "refs/tags/v1.0.0",
			want: git.Refspec("refs/tags/v1.0.0:refs/tags/v1.0.0"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRefspec(tt.src, tt.dst)
			if got != tt.want {
				t.Errorf("buildRefspec() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildBranchRef(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{
			name:   "main branch",
			branch: "main",
			want:   "refs/heads/main",
		},
		{
			name:   "feature branch",
			branch: "feature/new-feature",
			want:   "refs/heads/feature/new-feature",
		},
		{
			name:   "empty branch",
			branch: "",
			want:   "refs/heads/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildBranchRef(tt.branch)
			if got != tt.want {
				t.Errorf("buildBranchRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildRemoteBranchRef(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   string
	}{
		{
			name:   "main branch",
			branch: "main",
			want:   "refs/remotes/origin/main",
		},
		{
			name:   "feature branch",
			branch: "feature/new-feature",
			want:   "refs/remotes/origin/feature/new-feature",
		},
		{
			name:   "empty branch",
			branch: "",
			want:   "refs/remotes/origin/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := buildRemoteBranchRef(tt.branch)
			if got != tt.want {
				t.Errorf("buildRemoteBranchRef() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCreateTreeUpsert(t *testing.T) {
	hash := git.Hash("abc123def456")
	path := "protos/team/service/v1/api.proto"

	upsert := createTreeUpsert(path, hash)

	if upsert.Path != path {
		t.Errorf("createTreeUpsert() Path = %v, want %v", upsert.Path, path)
	}
	if upsert.Blob != hash {
		t.Errorf("createTreeUpsert() Blob = %v, want %v", upsert.Blob, hash)
	}
	if upsert.Mode != 0100644 {
		t.Errorf("createTreeUpsert() Mode = %v, want 0100644", upsert.Mode)
	}
}

func TestReadTreeError(t *testing.T) {
	originalErr := errors.New("git read-tree failed")
	wrappedErr := readTreeError(originalErr)

	if wrappedErr == nil {
		t.Fatal("readTreeError() returned nil")
	}

	errMsg := wrappedErr.Error()
	if errMsg == "" {
		t.Error("readTreeError() error message is empty")
	}
	if !errors.Is(wrappedErr, originalErr) {
		t.Error("readTreeError() does not wrap original error")
	}
}
