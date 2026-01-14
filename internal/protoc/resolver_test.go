package protoc

import (
	"context"
	"io"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/errors"
	"github.com/rahulagarwal0605/protato/internal/git"
	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/registry"
)

// mockCache is a mock implementation of CacheInterface for testing
type mockCache struct {
	lookupProjectFunc    func(ctx context.Context, req *registry.LookupProjectRequest) (*registry.LookupProjectResponse, error)
	listProjectFilesFunc func(ctx context.Context, req *registry.ListProjectFilesRequest) (*registry.ListProjectFilesResponse, error)
	readProjectFileFunc  func(ctx context.Context, file registry.ProjectFile, w io.Writer) error
}

func (m *mockCache) Close() error                                    { return nil }
func (m *mockCache) Refresh(context.Context) error                   { return nil }
func (m *mockCache) Snapshot(context.Context) (git.Hash, error)      { return git.Hash("abc123"), nil }
func (m *mockCache) URL() string                                     { return "https://example.com/registry.git" }
func (m *mockCache) GetSnapshot(context.Context) (git.Hash, error)  { return git.Hash("abc123"), nil }
func (m *mockCache) RefreshAndGetSnapshot(context.Context) (git.Hash, error) {
	return git.Hash("abc123"), nil
}
func (m *mockCache) Push(context.Context, git.Hash) error            { return nil }
func (m *mockCache) SetProject(context.Context, *registry.SetProjectRequest) (*registry.SetProjectResponse, error) {
	return nil, nil
}
func (m *mockCache) ListProjects(context.Context, *registry.ListProjectsOptions) ([]registry.ProjectPath, error) {
	return nil, nil
}
func (m *mockCache) CheckProjectClaim(context.Context, git.Hash, string, string) error {
	return nil
}

func (m *mockCache) LookupProject(ctx context.Context, req *registry.LookupProjectRequest) (*registry.LookupProjectResponse, error) {
	if m.lookupProjectFunc != nil {
		return m.lookupProjectFunc(ctx, req)
	}
	return nil, errors.ErrNotFound
}

func (m *mockCache) ListProjectFiles(ctx context.Context, req *registry.ListProjectFilesRequest) (*registry.ListProjectFilesResponse, error) {
	if m.listProjectFilesFunc != nil {
		return m.listProjectFilesFunc(ctx, req)
	}
	return nil, nil
}

func (m *mockCache) ReadProjectFile(ctx context.Context, file registry.ProjectFile, w io.Writer) error {
	if m.readProjectFileFunc != nil {
		return m.readProjectFileFunc(ctx, file, w)
	}
	return nil
}

func TestNewRegistryResolver(t *testing.T) {
	ctx := context.Background()
	cache := &mockCache{}
	snapshot := git.Hash("abc123")

	resolver := NewRegistryResolver(ctx, cache, snapshot)

	if resolver == nil {
		t.Fatal("NewRegistryResolver() returned nil")
	}
	if resolver.cache == nil {
		t.Error("NewRegistryResolver() cache is nil")
	}
	if resolver.snapshot != snapshot {
		t.Errorf("NewRegistryResolver() snapshot = %v, want %v", resolver.snapshot, snapshot)
	}
	if resolver.projects == nil {
		t.Error("NewRegistryResolver() projects map is nil")
	}
	if resolver.fileCache == nil {
		t.Error("NewRegistryResolver() fileCache map is nil")
	}
}

func TestRegistryResolver_SetImportPrefix(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	resolver.SetImportPrefix("proto")
	if resolver.importPrefix != "proto" {
		t.Errorf("SetImportPrefix() importPrefix = %v, want proto", resolver.importPrefix)
	}

	resolver.SetImportPrefix("")
	if resolver.importPrefix != "" {
		t.Errorf("SetImportPrefix() importPrefix = %v, want empty string", resolver.importPrefix)
	}
}

func TestRegistryResolver_SetServicePrefix(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	resolver.SetServicePrefix("test-service")
	if resolver.servicePrefix != "test-service" {
		t.Errorf("SetServicePrefix() servicePrefix = %v, want test-service", resolver.servicePrefix)
	}

	resolver.SetServicePrefix("")
	if resolver.servicePrefix != "" {
		t.Errorf("SetServicePrefix() servicePrefix = %v, want empty string", resolver.servicePrefix)
	}
}

func TestRegistryResolver_buildImportCachePath(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	tests := []struct {
		name      string
		prefix    string
		subPath   string
		want      string
	}{
		{
			name:    "with prefix",
			prefix:  "proto",
			subPath: "common/address.proto",
			want:    "proto/common/address.proto",
		},
		{
			name:    "without prefix",
			prefix:  "",
			subPath: "common/address.proto",
			want:    "common/address.proto",
		},
		{
			name:    "empty subPath",
			prefix:  "proto",
			subPath: "",
			want:    "proto/",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver.SetImportPrefix(tt.prefix)
			got := resolver.buildImportCachePath(tt.subPath)
			if got != tt.want {
				t.Errorf("buildImportCachePath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryResolver_mapImportPath(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	tests := []struct {
		name          string
		servicePrefix string
		importPrefix  string
		importPath    string
		want          string
	}{
		{
			name:          "google protobuf import",
			servicePrefix: "test-service",
			importPrefix:  "proto",
			importPath:    "google/protobuf/timestamp.proto",
			want:          "google/protobuf/timestamp.proto",
		},
		{
			name:          "import with service prefix",
			servicePrefix: "test-service",
			importPrefix:  "proto",
			importPath:    "test-service/common/address.proto",
			want:          "proto/common/address.proto",
		},
		{
			name:          "import without service prefix",
			servicePrefix: "test-service",
			importPrefix:  "proto",
			importPath:    "common/address.proto",
			want:          "common/address.proto",
		},
		{
			name:          "no service prefix configured",
			servicePrefix: "",
			importPrefix:  "proto",
			importPath:    "common/address.proto",
			want:          "common/address.proto",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			resolver.SetServicePrefix(tt.servicePrefix)
			resolver.SetImportPrefix(tt.importPrefix)
			got := resolver.mapImportPath(tt.importPath)
			if got != tt.want {
				t.Errorf("mapImportPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryResolver_DiscoveredProjects(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	// Initially empty
	projects := resolver.DiscoveredProjects()
	if len(projects) != 0 {
		t.Errorf("DiscoveredProjects() length = %v, want 0", len(projects))
	}

	// Register some projects
	resolver.registerProject(registry.ProjectPath("team/service1"))
	resolver.registerProject(registry.ProjectPath("team/service2"))

	projects = resolver.DiscoveredProjects()
	if len(projects) != 2 {
		t.Errorf("DiscoveredProjects() length = %v, want 2", len(projects))
	}

	projectMap := make(map[registry.ProjectPath]bool)
	for _, p := range projects {
		projectMap[p] = true
	}

	if !projectMap["team/service1"] {
		t.Error("DiscoveredProjects() missing team/service1")
	}
	if !projectMap["team/service2"] {
		t.Error("DiscoveredProjects() missing team/service2")
	}
}

func TestRegistryResolver_FindFileByPath_Preloaded(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))
	resolver.preloaded = true

	// Cache a file
	content := []byte("syntax = \"proto3\";")
	resolver.cacheFile("proto/common/address.proto", content)

	// Find cached file
	result, err := resolver.FindFileByPath("proto/common/address.proto")
	if err != nil {
		t.Fatalf("FindFileByPath() error = %v", err)
	}

	if result.Source == nil {
		t.Fatal("FindFileByPath() Source is nil")
	}

	// Read and verify content
	buf := make([]byte, len(content))
	n, err := result.Source.Read(buf)
	if err != nil && err.Error() != "EOF" {
		t.Fatalf("Read() error = %v", err)
	}
	if n != len(content) {
		t.Errorf("Read() length = %v, want %v", n, len(content))
	}
	if string(buf) != string(content) {
		t.Errorf("Read() content = %v, want %v", string(buf), string(content))
	}
}

func TestRegistryResolver_FindFileByPath_NotFound(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))
	resolver.preloaded = true

	// Try to find non-existent file
	_, err := resolver.FindFileByPath("proto/nonexistent.proto")
	if err == nil {
		t.Error("FindFileByPath() error = nil, want error")
	}
	if err != errors.ErrNotFound {
		t.Errorf("FindFileByPath() error = %v, want ErrNotFound", err)
	}
}

func TestRegistryResolver_FindFileByPath_NilResolver(t *testing.T) {
	var resolver *RegistryResolver
	_, err := resolver.FindFileByPath("proto/common/address.proto")
	if err == nil {
		t.Error("FindFileByPath() on nil resolver error = nil, want error")
	}
}

func TestIsGoogleProtobufImport(t *testing.T) {
	tests := []struct {
		name       string
		importPath string
		want       bool
	}{
		{
			name:       "google protobuf import",
			importPath: "google/protobuf/timestamp.proto",
			want:       true,
		},
		{
			name:       "google protobuf any",
			importPath: "google/protobuf/any.proto",
			want:       true,
		},
		{
			name:       "non-google import",
			importPath: "common/address.proto",
			want:       false,
		},
		{
			name:       "empty string",
			importPath: "",
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isGoogleProtobufImport(tt.importPath)
			if got != tt.want {
				t.Errorf("isGoogleProtobufImport() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRegistryResolver_PreloadFiles(t *testing.T) {
	log := logger.Init()
	ctx := logger.WithLogger(context.Background(), &log)
	cache := &mockCache{}

	var filesListed []registry.ProjectPath
	var filesRead []registry.ProjectFile

	cache.listProjectFilesFunc = func(ctx context.Context, req *registry.ListProjectFilesRequest) (*registry.ListProjectFilesResponse, error) {
		filesListed = append(filesListed, req.Project)
		return &registry.ListProjectFilesResponse{
			Files: []registry.ProjectFile{
				{Path: "v1/api.proto", Hash: git.Hash("hash1")},
				{Path: "v1/messages.proto", Hash: git.Hash("hash2")},
			},
		}, nil
	}

	cache.readProjectFileFunc = func(ctx context.Context, file registry.ProjectFile, w io.Writer) error {
		filesRead = append(filesRead, file)
		w.Write([]byte("syntax = \"proto3\";"))
		return nil
	}

	resolver := NewRegistryResolver(ctx, cache, git.Hash("abc123"))
	resolver.SetServicePrefix("test-service")
	resolver.SetImportPrefix("proto")

	projects := []registry.ProjectPath{"team/service1", "team/service2"}
	err := resolver.PreloadFiles(ctx, projects, false)
	if err != nil {
		t.Fatalf("PreloadFiles() error = %v", err)
	}

	if !resolver.preloaded {
		t.Error("PreloadFiles() preloaded = false, want true")
	}

	if len(filesListed) != len(projects) {
		t.Errorf("PreloadFiles() listed %v projects, want %v", len(filesListed), len(projects))
	}

	// Should have read 4 files (2 projects * 2 files each)
	if len(filesRead) != 4 {
		t.Errorf("PreloadFiles() read %v files, want 4", len(filesRead))
	}

	// Verify projects were registered
	discovered := resolver.DiscoveredProjects()
	if len(discovered) != len(projects) {
		t.Errorf("PreloadFiles() discovered %v projects, want %v", len(discovered), len(projects))
	}
}

func TestRegistryResolver_untransformImports(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))
	resolver.SetServicePrefix("test-service")
	resolver.SetImportPrefix("proto")

	tests := []struct {
		name    string
		content []byte
		want    string
	}{
		{
			name:    "import with service prefix",
			content: []byte("import \"test-service/common/address.proto\";"),
			want:    "import \"proto/common/address.proto\";",
		},
		{
			name:    "multiple imports",
			content: []byte("import \"test-service/common/address.proto\";\nimport \"test-service/common/types.proto\";"),
			want:    "import \"proto/common/address.proto\";\nimport \"proto/common/types.proto\";",
		},
		{
			name:    "no service prefix",
			content: []byte("import \"common/address.proto\";"),
			want:    "import \"common/address.proto\";",
		},
		{
			name:    "google protobuf import",
			content: []byte("import \"google/protobuf/timestamp.proto\";"),
			want:    "import \"google/protobuf/timestamp.proto\";",
		},
		{
			name:    "non-import line",
			content: []byte("syntax = \"proto3\";\npackage test;"),
			want:    "syntax = \"proto3\";\npackage test;",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := resolver.untransformImports(tt.content)
			if string(got) != tt.want {
				t.Errorf("untransformImports() = %v, want %v", string(got), tt.want)
			}
		})
	}
}

func TestRegistryResolver_untransformImports_NoServicePrefix(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))
	// No service prefix set

	content := []byte("import \"test-service/common/address.proto\";")
	got := resolver.untransformImports(content)

	// Should return unchanged when no service prefix
	if string(got) != string(content) {
		t.Errorf("untransformImports() = %v, want %v", string(got), string(content))
	}
}

func TestRegistryResolver_cacheFile(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	content := []byte("syntax = \"proto3\";")
	resolver.cacheFile("proto/common/address.proto", content)

	// Verify cached
	cached, ok := resolver.getCachedFile("proto/common/address.proto")
	if !ok {
		t.Error("getCachedFile() ok = false, want true")
	}
	if string(cached) != string(content) {
		t.Errorf("getCachedFile() content = %v, want %v", string(cached), string(content))
	}
}

func TestRegistryResolver_getCachedFile(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	// File not cached
	_, ok := resolver.getCachedFile("proto/nonexistent.proto")
	if ok {
		t.Error("getCachedFile() ok = true, want false")
	}

	// Cache a file
	content := []byte("syntax = \"proto3\";")
	resolver.cacheFile("proto/common/address.proto", content)

	// Get cached file
	cached, ok := resolver.getCachedFile("proto/common/address.proto")
	if !ok {
		t.Error("getCachedFile() ok = false, want true")
	}
	if string(cached) != string(content) {
		t.Errorf("getCachedFile() content = %v, want %v", string(cached), string(content))
	}
}

func TestRegistryResolver_registerProject(t *testing.T) {
	ctx := context.Background()
	resolver := NewRegistryResolver(ctx, &mockCache{}, git.Hash("abc123"))

	project := registry.ProjectPath("team/service")
	resolver.registerProject(project)

	projects := resolver.DiscoveredProjects()
	if len(projects) != 1 {
		t.Errorf("DiscoveredProjects() length = %v, want 1", len(projects))
	}
	if projects[0] != project {
		t.Errorf("DiscoveredProjects()[0] = %v, want %v", projects[0], project)
	}
}
