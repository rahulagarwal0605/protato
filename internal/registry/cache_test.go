package registry

import (
	"bytes"
	"context"
	"errors"
	"io"
	"testing"

	"github.com/rs/zerolog"

	"github.com/rahulagarwal0605/protato/internal/constants"
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
)

// testContext creates a context with a discarding logger for tests.
func testContext() context.Context {
	ctx := context.Background()
	log := zerolog.New(io.Discard)
	return logger.WithLogger(ctx, &log)
}

// mockRepository implements git.RepositoryInterface for testing.
type mockRepository struct {
	rootDir      string
	gitDir       string
	bare         bool
	fetchErr     error
	pushErr      error
	revHashErr   error
	revHashMap   map[string]git.Hash
	revExists    map[string]bool
	readTreeErr  error
	readTreeResp []git.TreeEntry
	writeObjErr  error
	writeObjHash git.Hash
	readObjErr   error
	readObjData  []byte
	updateTreeErr error
	updateTreeHash git.Hash
	commitTreeErr  error
	commitTreeHash git.Hash
	updateRefErr   error
	remoteURL     string
	remoteURLErr  error
	user         git.Author
	userErr      error
	repoURL      string
	repoURLErr   error
}

func (m *mockRepository) Root() string                           { return m.rootDir }
func (m *mockRepository) GitDir() string                         { return m.gitDir }
func (m *mockRepository) IsBare() bool                           { return m.bare }
func (m *mockRepository) Fetch(ctx context.Context, opts git.FetchOptions) error { return m.fetchErr }
func (m *mockRepository) Push(ctx context.Context, opts git.PushOptions) error { return m.pushErr }

func (m *mockRepository) RevHash(ctx context.Context, rev string) (git.Hash, error) {
	if m.revHashErr != nil {
		return "", m.revHashErr
	}
	if hash, ok := m.revHashMap[rev]; ok {
		return hash, nil
	}
	return "", errors.New("rev not found")
}

func (m *mockRepository) RevExists(ctx context.Context, rev string) bool {
	if m.revExists != nil {
		return m.revExists[rev]
	}
	_, ok := m.revHashMap[rev]
	return ok
}

func (m *mockRepository) ReadTree(ctx context.Context, tree git.Treeish, opts git.ReadTreeOptions) ([]git.TreeEntry, error) {
	if m.readTreeErr != nil {
		return nil, m.readTreeErr
	}
	return m.readTreeResp, nil
}

func (m *mockRepository) WriteObject(ctx context.Context, r io.Reader, opts git.WriteObjectOptions) (git.Hash, error) {
	if m.writeObjErr != nil {
		return "", m.writeObjErr
	}
	return m.writeObjHash, nil
}

func (m *mockRepository) ReadObject(ctx context.Context, objType git.ObjectType, hash git.Hash, w io.Writer) error {
	if m.readObjErr != nil {
		return m.readObjErr
	}
	if m.readObjData != nil {
		_, err := w.Write(m.readObjData)
		return err
	}
	return nil
}

func (m *mockRepository) UpdateTree(ctx context.Context, req git.UpdateTreeRequest) (git.Hash, error) {
	if m.updateTreeErr != nil {
		return "", m.updateTreeErr
	}
	return m.updateTreeHash, nil
}

func (m *mockRepository) CommitTree(ctx context.Context, req git.CommitTreeRequest) (git.Hash, error) {
	if m.commitTreeErr != nil {
		return "", m.commitTreeErr
	}
	return m.commitTreeHash, nil
}

func (m *mockRepository) UpdateRef(ctx context.Context, ref string, newHash, oldHash git.Hash) error {
	return m.updateRefErr
}

func (m *mockRepository) GetRemoteURL(ctx context.Context, remote string) (string, error) {
	if m.remoteURLErr != nil {
		return "", m.remoteURLErr
	}
	return m.remoteURL, nil
}

func (m *mockRepository) GetUser(ctx context.Context) (git.Author, error) {
	if m.userErr != nil {
		return git.Author{}, m.userErr
	}
	return m.user, nil
}

func (m *mockRepository) GetRepoURL(ctx context.Context) (string, error) {
	if m.repoURLErr != nil {
		return "", m.repoURLErr
	}
	return m.repoURL, nil
}

// newMockCache creates a Cache with a mock repository for testing.
func newMockCache(repo *mockRepository, url string) *Cache {
	return &Cache{
		root:     "/tmp/test-cache",
		repo:     repo,
		url:      url,
		lockFile: nil,
	}
}

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

func TestProjectPathJoin(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		parts  []string
		want   string
	}{
		{
			name:   "join with single part",
			prefix: "team/service",
			parts:  []string{"v1"},
			want:   "team/service/v1",
		},
		{
			name:   "join with multiple parts",
			prefix: "team",
			parts:  []string{"service", "v1", "api.proto"},
			want:   "team/service/v1/api.proto",
		},
		{
			name:   "join with no parts",
			prefix: "team/service",
			parts:  []string{},
			want:   "team/service",
		},
		{
			name:   "empty prefix with parts",
			prefix: "",
			parts:  []string{"team", "service"},
			want:   "team/service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := projectPathJoin(tt.prefix, tt.parts...)
			if got != tt.want {
				t.Errorf("projectPathJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_URL(t *testing.T) {
	repo := &mockRepository{}
	cache := newMockCache(repo, "https://github.com/test/registry.git")

	url := cache.URL()

	if url != "https://github.com/test/registry.git" {
		t.Errorf("URL() = %v, want %v", url, "https://github.com/test/registry.git")
	}
}

func TestCache_Close(t *testing.T) {
	repo := &mockRepository{}
	cache := newMockCache(repo, "https://github.com/test/registry.git")
	// lockFile is nil, so Close should return nil
	
	err := cache.Close()
	if err != nil {
		t.Errorf("Close() error = %v, want nil", err)
	}
}

func TestCache_Snapshot(t *testing.T) {
	tests := []struct {
		name       string
		revHashMap map[string]git.Hash
		revHashErr error
		wantHash   git.Hash
		wantErr    bool
	}{
		{
			name: "FETCH_HEAD exists",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "abc123",
			},
			wantHash: "abc123",
			wantErr:  false,
		},
		{
			name: "fallback to HEAD",
			revHashMap: map[string]git.Hash{
				"HEAD": "def456",
			},
			wantHash: "def456",
			wantErr:  false,
		},
		{
			name:       "both fail",
			revHashMap: map[string]git.Hash{},
			wantHash:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
				revHashErr: tt.revHashErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			hash, err := cache.Snapshot(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Snapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if hash != tt.wantHash {
				t.Errorf("Snapshot() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestCache_GetSnapshot(t *testing.T) {
	repo := &mockRepository{
		revHashMap: map[string]git.Hash{
			"FETCH_HEAD": "snapshot123",
		},
	}
	cache := newMockCache(repo, "https://github.com/test/registry.git")
	ctx := testContext()

	hash, err := cache.GetSnapshot(ctx)

	if err != nil {
		t.Errorf("GetSnapshot() error = %v", err)
	}
	if hash != "snapshot123" {
		t.Errorf("GetSnapshot() = %v, want snapshot123", hash)
	}
}

func TestCache_Refresh(t *testing.T) {
	tests := []struct {
		name     string
		fetchErr error
		wantErr  bool
	}{
		{
			name:     "successful refresh",
			fetchErr: nil,
			wantErr:  false,
		},
		{
			name:     "fetch error",
			fetchErr: errors.New("network error"),
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				fetchErr: tt.fetchErr,
				revHashMap: map[string]git.Hash{
					"HEAD": "abc123",
				},
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			err := cache.Refresh(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("Refresh() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_RefreshAndGetSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		fetchErr   error
		revHashMap map[string]git.Hash
		wantHash   git.Hash
		wantErr    bool
	}{
		{
			name:     "successful refresh and get",
			fetchErr: nil,
			revHashMap: map[string]git.Hash{
				"HEAD":       "abc123",
				"FETCH_HEAD": "def456",
			},
			wantHash: "def456",
			wantErr:  false,
		},
		{
			name:       "fetch error",
			fetchErr:   errors.New("network error"),
			revHashMap: map[string]git.Hash{},
			wantHash:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				fetchErr:   tt.fetchErr,
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			hash, err := cache.RefreshAndGetSnapshot(ctx)

			if (err != nil) != tt.wantErr {
				t.Errorf("RefreshAndGetSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && hash != tt.wantHash {
				t.Errorf("RefreshAndGetSnapshot() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestCache_getOrCreateSnapshot(t *testing.T) {
	tests := []struct {
		name       string
		snapshot   git.Hash
		revHashMap map[string]git.Hash
		wantHash   git.Hash
		wantErr    bool
	}{
		{
			name:       "use provided snapshot",
			snapshot:   "provided123",
			revHashMap: map[string]git.Hash{},
			wantHash:   "provided123",
			wantErr:    false,
		},
		{
			name:     "create new snapshot",
			snapshot: "",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "auto123",
			},
			wantHash: "auto123",
			wantErr:  false,
		},
		{
			name:       "error when no snapshot available",
			snapshot:   "",
			revHashMap: map[string]git.Hash{},
			wantHash:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			hash, err := cache.getOrCreateSnapshot(ctx, tt.snapshot)

			if (err != nil) != tt.wantErr {
				t.Errorf("getOrCreateSnapshot() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if hash != tt.wantHash {
				t.Errorf("getOrCreateSnapshot() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestCache_getDefaultBranch(t *testing.T) {
	tests := []struct {
		name       string
		revHashMap map[string]git.Hash
		revHashErr error
		want       string
	}{
		{
			name: "main branch detected",
			revHashMap: map[string]git.Hash{
				"HEAD":            "abc123",
				"refs/heads/main": "abc123",
			},
			want: "main",
		},
		{
			name: "master branch detected",
			revHashMap: map[string]git.Hash{
				"HEAD":              "abc123",
				"refs/heads/master": "abc123",
			},
			want: "master",
		},
		{
			name:       "default to main when HEAD fails",
			revHashMap: map[string]git.Hash{},
			revHashErr: errors.New("no HEAD"),
			want:       "main",
		},
		{
			name: "default to main when no branch matches",
			revHashMap: map[string]git.Hash{
				"HEAD": "abc123",
			},
			want: "main",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			got := cache.getDefaultBranch(ctx)

			if got != tt.want {
				t.Errorf("getDefaultBranch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_findBranchMatchingHash(t *testing.T) {
	tests := []struct {
		name       string
		hash       git.Hash
		revHashMap map[string]git.Hash
		want       string
	}{
		{
			name: "matches main",
			hash: "abc123",
			revHashMap: map[string]git.Hash{
				"refs/heads/main": "abc123",
			},
			want: "main",
		},
		{
			name: "matches master",
			hash: "abc123",
			revHashMap: map[string]git.Hash{
				"refs/heads/master": "abc123",
			},
			want: "master",
		},
		{
			name: "matches remote main",
			hash: "abc123",
			revHashMap: map[string]git.Hash{
				"refs/remotes/origin/main": "abc123",
			},
			want: "main",
		},
		{
			name:       "no match",
			hash:       "abc123",
			revHashMap: map[string]git.Hash{},
			want:       "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			got := cache.findBranchMatchingHash(ctx, tt.hash)

			if got != tt.want {
				t.Errorf("findBranchMatchingHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_branchMatchesHash(t *testing.T) {
	tests := []struct {
		name       string
		branch     string
		hash       git.Hash
		revHashMap map[string]git.Hash
		want       bool
	}{
		{
			name:   "local branch matches",
			branch: "main",
			hash:   "abc123",
			revHashMap: map[string]git.Hash{
				"refs/heads/main": "abc123",
			},
			want: true,
		},
		{
			name:   "remote branch matches",
			branch: "main",
			hash:   "abc123",
			revHashMap: map[string]git.Hash{
				"refs/remotes/origin/main": "abc123",
			},
			want: true,
		},
		{
			name:       "no match",
			branch:     "main",
			hash:       "abc123",
			revHashMap: map[string]git.Hash{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			got := cache.branchMatchesHash(ctx, tt.branch, tt.hash)

			if got != tt.want {
				t.Errorf("branchMatchesHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_checkHashMatch(t *testing.T) {
	tests := []struct {
		name       string
		rev        string
		hash       git.Hash
		revHashMap map[string]git.Hash
		want       bool
	}{
		{
			name: "hash matches",
			rev:  "refs/heads/main",
			hash: "abc123",
			revHashMap: map[string]git.Hash{
				"refs/heads/main": "abc123",
			},
			want: true,
		},
		{
			name: "hash does not match",
			rev:  "refs/heads/main",
			hash: "abc123",
			revHashMap: map[string]git.Hash{
				"refs/heads/main": "def456",
			},
			want: false,
		},
		{
			name:       "rev not found",
			rev:        "refs/heads/main",
			hash:       "abc123",
			revHashMap: map[string]git.Hash{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: tt.revHashMap,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			got := cache.checkHashMatch(ctx, tt.rev, tt.hash)

			if got != tt.want {
				t.Errorf("checkHashMatch() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_Push(t *testing.T) {
	tests := []struct {
		name     string
		hash     git.Hash
		pushErr  error
		wantErr  bool
	}{
		{
			name:    "successful push",
			hash:    "abc123",
			pushErr: nil,
			wantErr: false,
		},
		{
			name:    "push error",
			hash:    "abc123",
			pushErr: errors.New("push failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				pushErr: tt.pushErr,
				revHashMap: map[string]git.Hash{
					"HEAD": "def456",
				},
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			err := cache.Push(ctx, tt.hash)

			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_writeObject(t *testing.T) {
	tests := []struct {
		name        string
		writeObjErr error
		writeObjHash git.Hash
		wantHash    git.Hash
		wantErr     bool
	}{
		{
			name:        "successful write",
			writeObjErr: nil,
			writeObjHash: "abc123",
			wantHash:    "abc123",
			wantErr:     false,
		},
		{
			name:        "write error",
			writeObjErr: errors.New("write failed"),
			writeObjHash: "",
			wantHash:    "",
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				writeObjErr:  tt.writeObjErr,
				writeObjHash: tt.writeObjHash,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			hash, err := cache.writeObject(ctx, bytes.NewReader([]byte("test content")))

			if (err != nil) != tt.wantErr {
				t.Errorf("writeObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if hash != tt.wantHash {
				t.Errorf("writeObject() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestCache_ListProjects(t *testing.T) {
	tests := []struct {
		name         string
		opts         *ListProjectsOptions
		revHashMap   map[string]git.Hash
		readTreeResp []git.TreeEntry
		readTreeErr  error
		wantLen      int
		wantErr      bool
	}{
		{
			name: "list all projects",
			opts: nil,
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType},
				{Path: constants.ProtosDir + "/team/service2/" + constants.ProjectMetaFile, Type: git.BlobType},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name: "list with prefix",
			opts: &ListProjectsOptions{
				Prefix: "team",
			},
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "filter out non-blob entries",
			opts: nil,
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType},
				{Path: constants.ProtosDir + "/team/service", Type: git.TreeType},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name: "read tree error",
			opts: nil,
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeErr: errors.New("read tree failed"),
			wantLen:     0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:   tt.revHashMap,
				readTreeResp: tt.readTreeResp,
				readTreeErr:  tt.readTreeErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			projects, err := cache.ListProjects(ctx, tt.opts)

			if (err != nil) != tt.wantErr {
				t.Errorf("ListProjects() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(projects) != tt.wantLen {
				t.Errorf("ListProjects() returned %d projects, want %d", len(projects), tt.wantLen)
			}
		})
	}
}

func TestCache_ListProjectFiles(t *testing.T) {
	tests := []struct {
		name         string
		project      ProjectPath
		revHashMap   map[string]git.Hash
		readTreeResp []git.TreeEntry
		readTreeErr  error
		wantLen      int
		wantErr      bool
	}{
		{
			name:    "list project files",
			project: "team/service",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/api.proto", Type: git.BlobType, Hash: "hash1"},
				{Path: constants.ProtosDir + "/team/service/types.proto", Type: git.BlobType, Hash: "hash2"},
			},
			wantLen: 2,
			wantErr: false,
		},
		{
			name:    "filter non-proto files",
			project: "team/service",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/api.proto", Type: git.BlobType, Hash: "hash1"},
				{Path: constants.ProtosDir + "/team/service/.protato.yaml", Type: git.BlobType, Hash: "hash2"},
			},
			wantLen: 1,
			wantErr: false,
		},
		{
			name:    "read tree error",
			project: "team/service",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeErr: errors.New("read tree failed"),
			wantLen:     0,
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:   tt.revHashMap,
				readTreeResp: tt.readTreeResp,
				readTreeErr:  tt.readTreeErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			resp, err := cache.ListProjectFiles(ctx, &ListProjectFilesRequest{
				Project:  tt.project,
				Snapshot: "",
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("ListProjectFiles() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(resp.Files) != tt.wantLen {
				t.Errorf("ListProjectFiles() returned %d files, want %d", len(resp.Files), tt.wantLen)
			}
		})
	}
}

func TestCache_ReadProjectFile(t *testing.T) {
	tests := []struct {
		name       string
		file       ProjectFile
		readObjErr error
		readObjData []byte
		wantData   string
		wantErr    bool
	}{
		{
			name: "successful read",
			file: ProjectFile{
				Project: "team/service",
				Path:    "api.proto",
				Hash:    "abc123",
			},
			readObjErr:  nil,
			readObjData: []byte("syntax = \"proto3\";"),
			wantData:    "syntax = \"proto3\";",
			wantErr:     false,
		},
		{
			name: "read error",
			file: ProjectFile{
				Project: "team/service",
				Path:    "api.proto",
				Hash:    "abc123",
			},
			readObjErr: errors.New("read failed"),
			wantData:   "",
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				readObjErr:  tt.readObjErr,
				readObjData: tt.readObjData,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			var buf bytes.Buffer
			err := cache.ReadProjectFile(ctx, tt.file, &buf)

			if (err != nil) != tt.wantErr {
				t.Errorf("ReadProjectFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && buf.String() != tt.wantData {
				t.Errorf("ReadProjectFile() data = %v, want %v", buf.String(), tt.wantData)
			}
		})
	}
}

func TestCache_LookupProject(t *testing.T) {
	tests := []struct {
		name         string
		path         string
		snapshot     git.Hash
		revHashMap   map[string]git.Hash
		revExists    map[string]bool
		readTreeResp []git.TreeEntry
		readObjData  []byte
		wantErr      bool
	}{
		{
			name:     "project not found",
			path:     "team/service",
			snapshot: "",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			revExists: map[string]bool{
				"snapshot123": true,
			},
			readTreeResp: []git.TreeEntry{},
			wantErr:      true,
		},
		{
			name:     "snapshot not found",
			path:     "team/service",
			snapshot: "invalid",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			revExists: map[string]bool{
				"snapshot123": true,
				"invalid":     false,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:   tt.revHashMap,
				revExists:    tt.revExists,
				readTreeResp: tt.readTreeResp,
				readObjData:  tt.readObjData,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			_, err := cache.LookupProject(ctx, &LookupProjectRequest{
				Path:     tt.path,
				Snapshot: tt.snapshot,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("LookupProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_prepareDeletes(t *testing.T) {
	tests := []struct {
		name          string
		projectPath   ProjectPath
		newFiles      []LocalProjectFile
		readTreeResp  []git.TreeEntry
		wantDeletes   int
	}{
		{
			name:        "no existing files",
			projectPath: "team/service",
			newFiles:    []LocalProjectFile{{Path: "api.proto"}},
			readTreeResp: []git.TreeEntry{},
			wantDeletes:  0,
		},
		{
			name:        "files to delete",
			projectPath: "team/service",
			newFiles:    []LocalProjectFile{{Path: "api.proto"}},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/api.proto", Type: git.BlobType},
				{Path: constants.ProtosDir + "/team/service/old.proto", Type: git.BlobType},
			},
			wantDeletes: 1,
		},
		{
			name:        "no files to delete",
			projectPath: "team/service",
			newFiles:    []LocalProjectFile{{Path: "api.proto"}, {Path: "types.proto"}},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/api.proto", Type: git.BlobType},
				{Path: constants.ProtosDir + "/team/service/types.proto", Type: git.BlobType},
			},
			wantDeletes: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap: map[string]git.Hash{
					"FETCH_HEAD": "snapshot123",
				},
				readTreeResp: tt.readTreeResp,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			deletes, err := cache.prepareDeletes(ctx, tt.projectPath, tt.newFiles, "snapshot123", protosPath(string(tt.projectPath)))

			if err != nil {
				t.Errorf("prepareDeletes() error = %v", err)
				return
			}
			if len(deletes) != tt.wantDeletes {
				t.Errorf("prepareDeletes() returned %d deletes, want %d", len(deletes), tt.wantDeletes)
			}
		})
	}
}

func TestCache_createProjectCommit(t *testing.T) {
	tests := []struct {
		name           string
		author         *git.Author
		commitTreeHash git.Hash
		commitTreeErr  error
		wantErr        bool
	}{
		{
			name:           "successful commit",
			author:         &git.Author{Name: "Test User", Email: "test@example.com"},
			commitTreeHash: "newcommit123",
			commitTreeErr:  nil,
			wantErr:        false,
		},
		{
			name:           "missing author",
			author:         nil,
			commitTreeHash: "",
			commitTreeErr:  nil,
			wantErr:        true,
		},
		{
			name:           "commit error",
			author:         &git.Author{Name: "Test User", Email: "test@example.com"},
			commitTreeHash: "",
			commitTreeErr:  errors.New("commit failed"),
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				commitTreeHash: tt.commitTreeHash,
				commitTreeErr:  tt.commitTreeErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			req := &SetProjectRequest{
				Project: &Project{Path: "team/service"},
				Files:   []LocalProjectFile{{Path: "api.proto"}},
				Author:  tt.author,
			}

			_, err := cache.createProjectCommit(ctx, req, "snapshot123", "tree123")

			if (err != nil) != tt.wantErr {
				t.Errorf("createProjectCommit() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestProjectPath_String(t *testing.T) {
	tests := []struct {
		name string
		path ProjectPath
		want string
	}{
		{
			name: "simple path",
			path: ProjectPath("team/service"),
			want: "team/service",
		},
		{
			name: "nested path",
			path: ProjectPath("org/team/service/v1"),
			want: "org/team/service/v1",
		},
		{
			name: "empty path",
			path: ProjectPath(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tt.path.String()
			if got != tt.want {
				t.Errorf("ProjectPath.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestCache_CheckProjectClaim(t *testing.T) {
	tests := []struct {
		name         string
		snapshot     git.Hash
		repoURL      string
		projectPath  string
		revHashMap   map[string]git.Hash
		revExists    map[string]bool
		readTreeResp []git.TreeEntry
		readObjData  []byte
		wantErr      bool
	}{
		{
			name:        "project not found - no subprojects",
			snapshot:    "snapshot123",
			repoURL:     "https://github.com/test/repo.git",
			projectPath: "team/service",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			revExists: map[string]bool{
				"snapshot123": true,
			},
			readTreeResp: []git.TreeEntry{},
			wantErr:      false,
		},
		{
			name:        "project not found - has subprojects",
			snapshot:    "snapshot123",
			repoURL:     "https://github.com/test/repo.git",
			projectPath: "team",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			revExists: map[string]bool{
				"snapshot123": true,
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:   tt.revHashMap,
				revExists:    tt.revExists,
				readTreeResp: tt.readTreeResp,
				readObjData:  tt.readObjData,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			err := cache.CheckProjectClaim(ctx, tt.snapshot, tt.repoURL, tt.projectPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("CheckProjectClaim() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_checkSubprojectConflicts(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		revHashMap   map[string]git.Hash
		readTreeResp []git.TreeEntry
		wantErr      bool
	}{
		{
			name:        "no subprojects",
			projectPath: "team/service",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{},
			wantErr:      false,
		},
		{
			name:        "has subprojects",
			projectPath: "team",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:   tt.revHashMap,
				readTreeResp: tt.readTreeResp,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			err := cache.checkSubprojectConflicts(ctx, "snapshot123", tt.projectPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("checkSubprojectConflicts() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_validateOwnership(t *testing.T) {
	tests := []struct {
		name        string
		response    *LookupProjectResponse
		repoURL     string
		projectPath string
		wantErr     bool
	}{
		{
			name: "same project path and URL",
			response: &LookupProjectResponse{
				Project: &Project{
					Path:          "team/service",
					RepositoryURL: "https://github.com/test/repo.git",
				},
			},
			repoURL:     "https://github.com/test/repo.git",
			projectPath: "team/service",
			wantErr:     false,
		},
		{
			name: "different URL - ownership conflict",
			response: &LookupProjectResponse{
				Project: &Project{
					Path:          "team/service",
					RepositoryURL: "https://github.com/other/repo.git",
				},
			},
			repoURL:     "https://github.com/test/repo.git",
			projectPath: "team/service",
			wantErr:     true,
		},
		{
			name: "parent project exists",
			response: &LookupProjectResponse{
				Project: &Project{
					Path:          "team",
					RepositoryURL: "https://github.com/test/repo.git",
				},
			},
			repoURL:     "https://github.com/test/repo.git",
			projectPath: "team/service",
			wantErr:     true,
		},
		{
			name: "empty repoURL - no ownership check",
			response: &LookupProjectResponse{
				Project: &Project{
					Path:          "team/service",
					RepositoryURL: "https://github.com/other/repo.git",
				},
			},
			repoURL:     "",
			projectPath: "team/service",
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			err := cache.validateOwnership(ctx, tt.response, tt.repoURL, tt.projectPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("validateOwnership() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_tryFindProjectAtPath(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		readTreeResp []git.TreeEntry
		readObjData  []byte
		readObjErr   error
		wantNil      bool
	}{
		{
			name:         "project not found",
			projectPath:  "team/service",
			readTreeResp: []git.TreeEntry{},
			wantNil:      true,
		},
		{
			name:        "project found with valid meta",
			projectPath: "team/service",
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType, Hash: "metahash"},
			},
			readObjData: []byte("git:\n  commit: abc123\n  url: https://github.com/test/repo.git\n"),
			wantNil:     false,
		},
		{
			name:        "project found but meta read fails",
			projectPath: "team/service",
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service/" + constants.ProjectMetaFile, Type: git.BlobType, Hash: "metahash"},
			},
			readObjErr: errors.New("read failed"),
			wantNil:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				readTreeResp: tt.readTreeResp,
				readObjData:  tt.readObjData,
				readObjErr:   tt.readObjErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			result := cache.tryFindProjectAtPath(ctx, "snapshot123", tt.projectPath)

			if (result == nil) != tt.wantNil {
				t.Errorf("tryFindProjectAtPath() returned nil = %v, want nil = %v", result == nil, tt.wantNil)
			}
		})
	}
}

func TestCache_getProjectTreeHash(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		readTreeResp []git.TreeEntry
		readTreeErr  error
		wantHash     git.Hash
	}{
		{
			name:        "tree hash found",
			projectPath: "team/service",
			readTreeResp: []git.TreeEntry{
				{Path: constants.ProtosDir + "/team/service", Type: git.TreeType, Hash: "treehash123"},
			},
			wantHash: "treehash123",
		},
		{
			name:         "tree hash not found",
			projectPath:  "team/service",
			readTreeResp: []git.TreeEntry{},
			wantHash:     "",
		},
		{
			name:        "read tree error",
			projectPath: "team/service",
			readTreeErr: errors.New("read tree failed"),
			wantHash:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				readTreeResp: tt.readTreeResp,
				readTreeErr:  tt.readTreeErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			hash := cache.getProjectTreeHash(ctx, "snapshot123", tt.projectPath)

			if hash != tt.wantHash {
				t.Errorf("getProjectTreeHash() = %v, want %v", hash, tt.wantHash)
			}
		})
	}
}

func TestCache_readProjectMeta(t *testing.T) {
	tests := []struct {
		name        string
		hash        git.Hash
		readObjData []byte
		readObjErr  error
		wantCommit  git.Hash
		wantURL     string
		wantErr     bool
	}{
		{
			name:        "valid meta",
			hash:        "metahash",
			readObjData: []byte("git:\n  commit: abc123\n  url: https://github.com/test/repo.git\n"),
			wantCommit:  "abc123",
			wantURL:     "https://github.com/test/repo.git",
			wantErr:     false,
		},
		{
			name:       "read error",
			hash:       "metahash",
			readObjErr: errors.New("read failed"),
			wantErr:    true,
		},
		{
			name:        "invalid yaml",
			hash:        "metahash",
			readObjData: []byte("invalid: [yaml"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				readObjData: tt.readObjData,
				readObjErr:  tt.readObjErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			project, err := cache.readProjectMeta(ctx, tt.hash)

			if (err != nil) != tt.wantErr {
				t.Errorf("readProjectMeta() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if project.Commit != tt.wantCommit {
					t.Errorf("readProjectMeta() commit = %v, want %v", project.Commit, tt.wantCommit)
				}
				if project.RepositoryURL != tt.wantURL {
					t.Errorf("readProjectMeta() url = %v, want %v", project.RepositoryURL, tt.wantURL)
				}
			}
		})
	}
}

func TestCache_findProjectByPath(t *testing.T) {
	tests := []struct {
		name         string
		projectPath  string
		readTreeResp []git.TreeEntry
		readObjData  []byte
		wantErr      bool
	}{
		{
			name:         "project not found at any level",
			projectPath:  "team/service/v1",
			readTreeResp: []git.TreeEntry{},
			wantErr:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				readTreeResp: tt.readTreeResp,
				readObjData:  tt.readObjData,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			_, err := cache.findProjectByPath(ctx, "snapshot123", tt.projectPath)

			if (err != nil) != tt.wantErr {
				t.Errorf("findProjectByPath() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestCache_prepareUpserts(t *testing.T) {
	tests := []struct {
		name         string
		project      *Project
		files        []LocalProjectFile
		writeObjHash git.Hash
		writeObjErr  error
		wantLen      int
		wantErr      bool
	}{
		{
			name: "successful upserts with content",
			project: &Project{
				Commit:        "abc123",
				RepositoryURL: "https://github.com/test/repo.git",
			},
			files: []LocalProjectFile{
				{Path: "api.proto", Content: []byte("syntax = \"proto3\";")},
			},
			writeObjHash: "newhash",
			wantLen:      2, // meta + 1 file
			wantErr:      false,
		},
		{
			name: "write object error",
			project: &Project{
				Commit:        "abc123",
				RepositoryURL: "https://github.com/test/repo.git",
			},
			files:       []LocalProjectFile{},
			writeObjErr: errors.New("write failed"),
			wantErr:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				writeObjHash: tt.writeObjHash,
				writeObjErr:  tt.writeObjErr,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			upserts, err := cache.prepareUpserts(ctx, tt.project, tt.files, "protos/team/service")

			if (err != nil) != tt.wantErr {
				t.Errorf("prepareUpserts() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(upserts) != tt.wantLen {
				t.Errorf("prepareUpserts() returned %d upserts, want %d", len(upserts), tt.wantLen)
			}
		})
	}
}

func TestCache_SetProject(t *testing.T) {
	tests := []struct {
		name           string
		revHashMap     map[string]git.Hash
		writeObjHash   git.Hash
		updateTreeHash git.Hash
		updateTreeErr  error
		commitTreeHash git.Hash
		author         *git.Author
		wantErr        bool
	}{
		{
			name: "successful set project",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD":         "snapshot123",
				"snapshot123^{tree}": "treehash",
			},
			writeObjHash:   "newhash",
			updateTreeHash: "newtree",
			commitTreeHash: "newcommit",
			author:         &git.Author{Name: "Test User", Email: "test@example.com"},
			wantErr:        false,
		},
		{
			name: "get current tree error",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD": "snapshot123",
			},
			author:  &git.Author{Name: "Test User", Email: "test@example.com"},
			wantErr: true,
		},
		{
			name: "update tree error",
			revHashMap: map[string]git.Hash{
				"FETCH_HEAD":         "snapshot123",
				"snapshot123^{tree}": "treehash",
			},
			writeObjHash:  "newhash",
			updateTreeErr: errors.New("update tree failed"),
			author:        &git.Author{Name: "Test User", Email: "test@example.com"},
			wantErr:       true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockRepository{
				revHashMap:     tt.revHashMap,
				writeObjHash:   tt.writeObjHash,
				updateTreeHash: tt.updateTreeHash,
				updateTreeErr:  tt.updateTreeErr,
				commitTreeHash: tt.commitTreeHash,
			}
			cache := newMockCache(repo, "https://github.com/test/registry.git")
			ctx := testContext()

			_, err := cache.SetProject(ctx, &SetProjectRequest{
				Project: &Project{
					Path:          "team/service",
					Commit:        "abc123",
					RepositoryURL: "https://github.com/test/repo.git",
				},
				Files:  []LocalProjectFile{{Path: "api.proto", Content: []byte("syntax = \"proto3\";")}},
				Author: tt.author,
			})

			if (err != nil) != tt.wantErr {
				t.Errorf("SetProject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
