package git

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rs/zerolog"
)

// testContext creates a context with a discarding logger for tests.
func testContext() context.Context {
	log := zerolog.New(io.Discard)
	return logger.WithLogger(context.Background(), &log)
}

// =============================================================================
// Mock Execer for Testing
// =============================================================================

type mockExecer struct {
	runErr     error
	output     []byte
	outputErr  error
	outputFunc func() ([]byte, error)
}

func (m *mockExecer) Run(cmd *exec.Cmd) error {
	return m.runErr
}

func (m *mockExecer) Output(cmd *exec.Cmd) ([]byte, error) {
	if m.outputFunc != nil {
		return m.outputFunc()
	}
	return m.output, m.outputErr
}

// =============================================================================
// Hash Tests
// =============================================================================

func TestHash_String(t *testing.T) {
	tests := []struct {
		name string
		hash Hash
		want string
	}{
		{
			name: "normal hash",
			hash: Hash("abc123def456789"),
			want: "abc123def456789",
		},
		{
			name: "empty hash",
			hash: Hash(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.String(); got != tt.want {
				t.Errorf("Hash.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHash_Short(t *testing.T) {
	tests := []struct {
		name string
		hash Hash
		want string
	}{
		{
			name: "long hash",
			hash: Hash("abc123def456789"),
			want: "abc123d",
		},
		{
			name: "exactly 7 chars",
			hash: Hash("abc123d"),
			want: "abc123d",
		},
		{
			name: "short hash",
			hash: Hash("abc"),
			want: "abc",
		},
		{
			name: "empty hash",
			hash: Hash(""),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.hash.Short(); got != tt.want {
				t.Errorf("Hash.Short() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Treeish Tests
// =============================================================================

func TestTreeish_String(t *testing.T) {
	tests := []struct {
		name    string
		treeish Treeish
		want    string
	}{
		{
			name:    "commit ref",
			treeish: Treeish("HEAD"),
			want:    "HEAD",
		},
		{
			name:    "hash ref",
			treeish: Treeish("abc123"),
			want:    "abc123",
		},
		{
			name:    "empty ref",
			treeish: Treeish(""),
			want:    "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.treeish.String(); got != tt.want {
				t.Errorf("Treeish.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// ObjectType Tests
// =============================================================================

func TestObjectType_String(t *testing.T) {
	tests := []struct {
		name       string
		objectType ObjectType
		want       string
	}{
		{
			name:       "blob type",
			objectType: BlobType,
			want:       "blob",
		},
		{
			name:       "tree type",
			objectType: TreeType,
			want:       "tree",
		},
		{
			name:       "commit type",
			objectType: CommitType,
			want:       "commit",
		},
		{
			name:       "tag type",
			objectType: TagType,
			want:       "tag",
		},
		{
			name:       "unknown type",
			objectType: ObjectType(99),
			want:       "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.objectType.String(); got != tt.want {
				t.Errorf("ObjectType.String() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseObjectType(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    ObjectType
		wantErr bool
	}{
		{
			name:    "parse blob",
			input:   "blob",
			want:    BlobType,
			wantErr: false,
		},
		{
			name:    "parse tree",
			input:   "tree",
			want:    TreeType,
			wantErr: false,
		},
		{
			name:    "parse commit",
			input:   "commit",
			want:    CommitType,
			wantErr: false,
		},
		{
			name:    "parse tag",
			input:   "tag",
			want:    TagType,
			wantErr: false,
		},
		{
			name:    "parse unknown",
			input:   "unknown",
			wantErr: true,
		},
		{
			name:    "parse empty",
			input:   "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseObjectType(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseObjectType() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseObjectType() = %v, want %v", got, tt.want)
			}
		})
	}
}

// =============================================================================
// Repository Helper Function Tests
// =============================================================================

func TestTrimOutputToHash(t *testing.T) {
	tests := []struct {
		name string
		out  []byte
		want Hash
	}{
		{
			name: "normal hash",
			out:  []byte("abc123def456\n"),
			want: Hash("abc123def456"),
		},
		{
			name: "hash with spaces",
			out:  []byte("  abc123def456  \n"),
			want: Hash("abc123def456"),
		},
		{
			name: "hash without newline",
			out:  []byte("abc123def456"),
			want: Hash("abc123def456"),
		},
		{
			name: "empty output",
			out:  []byte(""),
			want: Hash(""),
		},
		{
			name: "whitespace only",
			out:  []byte("   \n  "),
			want: Hash(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := trimOutputToHash(tt.out)
			if got != tt.want {
				t.Errorf("trimOutputToHash() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAppendRefspecs(t *testing.T) {
	tests := []struct {
		name     string
		args     []string
		refspecs []Refspec
		want     []string
	}{
		{
			name:     "single refspec",
			args:     []string{"fetch"},
			refspecs: []Refspec{"refs/heads/main:refs/remotes/origin/main"},
			want:     []string{"fetch", "refs/heads/main:refs/remotes/origin/main"},
		},
		{
			name:     "multiple refspecs",
			args:     []string{"fetch"},
			refspecs: []Refspec{"refs/heads/main:refs/remotes/origin/main", "refs/heads/develop:refs/remotes/origin/develop"},
			want:     []string{"fetch", "refs/heads/main:refs/remotes/origin/main", "refs/heads/develop:refs/remotes/origin/develop"},
		},
		{
			name:     "no refspecs",
			args:     []string{"fetch"},
			refspecs: []Refspec{},
			want:     []string{"fetch"},
		},
		{
			name:     "empty args",
			args:     []string{},
			refspecs: []Refspec{"refs/heads/main:refs/remotes/origin/main"},
			want:     []string{"refs/heads/main:refs/remotes/origin/main"},
		},
		{
			name:     "nil refspecs",
			args:     []string{"push"},
			refspecs: nil,
			want:     []string{"push"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := appendRefspecs(tt.args, tt.refspecs)
			if len(got) != len(tt.want) {
				t.Fatalf("appendRefspecs() length = %v, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("appendRefspecs()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestAppendEnvToCmd(t *testing.T) {
	t.Run("append to existing env", func(t *testing.T) {
		cmd := &gitCmd{
			args: []string{"git", "command"},
			env:  []string{"VAR1=value1"},
		}

		env := []string{"VAR2=value2", "VAR3=value3"}
		appendEnvToCmd(cmd, env)

		if len(cmd.env) != 3 {
			t.Fatalf("appendEnvToCmd() env length = %v, want 3", len(cmd.env))
		}

		expected := []string{"VAR1=value1", "VAR2=value2", "VAR3=value3"}
		for i := range cmd.env {
			if cmd.env[i] != expected[i] {
				t.Errorf("appendEnvToCmd() env[%d] = %v, want %v", i, cmd.env[i], expected[i])
			}
		}
	})

	t.Run("append empty env", func(t *testing.T) {
		cmd := &gitCmd{
			args: []string{"git", "command"},
			env:  []string{"VAR1=value1"},
		}

		appendEnvToCmd(cmd, []string{})

		if len(cmd.env) != 1 {
			t.Errorf("appendEnvToCmd() env length = %v, want 1", len(cmd.env))
		}
	})

	t.Run("append to nil env", func(t *testing.T) {
		cmd := &gitCmd{
			args: []string{"git", "command"},
			env:  nil,
		}

		env := []string{"VAR1=value1"}
		appendEnvToCmd(cmd, env)

		if len(cmd.env) != 1 {
			t.Errorf("appendEnvToCmd() env length = %v, want 1", len(cmd.env))
		}
	})
}

func TestErrNotGitRepository(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "normal path",
			path: "/path/to/non-git/dir",
			want: "not a git repository: /path/to/non-git/dir",
		},
		{
			name: "empty path",
			path: "",
			want: "not a git repository: ",
		},
		{
			name: "relative path",
			path: "relative/path",
			want: "not a git repository: relative/path",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := errNotGitRepository(tt.path)
			if err == nil {
				t.Fatal("errNotGitRepository() returned nil")
			}
			if err.Error() != tt.want {
				t.Errorf("errNotGitRepository() error = %v, want %v", err.Error(), tt.want)
			}
		})
	}
}

// =============================================================================
// gitCmd Tests
// =============================================================================

func TestNewGitCmd(t *testing.T) {
	tests := []struct {
		name string
		args []string
	}{
		{
			name: "single arg",
			args: []string{"status"},
		},
		{
			name: "multiple args",
			args: []string{"commit", "-m", "message"},
		},
		{
			name: "no args",
			args: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cmd := newGitCmd(tt.args...)
			if cmd == nil {
				t.Fatal("newGitCmd() returned nil")
			}
			if len(cmd.args) != len(tt.args) {
				t.Errorf("newGitCmd() args length = %v, want %v", len(cmd.args), len(tt.args))
			}
			for i, arg := range tt.args {
				if cmd.args[i] != arg {
					t.Errorf("newGitCmd() args[%d] = %v, want %v", i, cmd.args[i], arg)
				}
			}
		})
	}
}

func TestWithExecer(t *testing.T) {
	ctx := context.Background()
	mock := &mockExecer{}

	newCtx := WithExecer(ctx, mock)

	got := GetExecer(newCtx)
	if got != mock {
		t.Error("WithExecer/GetExecer did not preserve the execer")
	}
}

func TestGetExecer_Default(t *testing.T) {
	ctx := context.Background()

	got := GetExecer(ctx)
	if got == nil {
		t.Fatal("GetExecer returned nil")
	}
	if _, ok := got.(*DefaultExecer); !ok {
		t.Error("GetExecer did not return DefaultExecer for context without execer")
	}
}

func TestDefaultExecer_Run(t *testing.T) {
	e := &DefaultExecer{}

	t.Run("successful command", func(t *testing.T) {
		cmd := exec.Command("echo", "test")
		err := e.Run(cmd)
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	})

	t.Run("failed command", func(t *testing.T) {
		cmd := exec.Command("false")
		err := e.Run(cmd)
		if err == nil {
			t.Error("Run() expected error for failed command")
		}
	})
}

func TestDefaultExecer_Output(t *testing.T) {
	e := &DefaultExecer{}

	t.Run("successful command", func(t *testing.T) {
		cmd := exec.Command("echo", "hello")
		out, err := e.Output(cmd)
		if err != nil {
			t.Errorf("Output() error = %v, want nil", err)
		}
		if !bytes.Contains(out, []byte("hello")) {
			t.Errorf("Output() = %s, want to contain 'hello'", out)
		}
	})
}

func TestRepository_Root(t *testing.T) {
	repo := &Repository{
		rootDir: "/path/to/repo",
		gitDir:  "/path/to/repo/.git",
	}

	if got := repo.Root(); got != "/path/to/repo" {
		t.Errorf("Root() = %v, want /path/to/repo", got)
	}
}

func TestRepository_GitDir(t *testing.T) {
	repo := &Repository{
		rootDir: "/path/to/repo",
		gitDir:  "/path/to/repo/.git",
	}

	if got := repo.GitDir(); got != "/path/to/repo/.git" {
		t.Errorf("GitDir() = %v, want /path/to/repo/.git", got)
	}
}

func TestRepository_IsBare(t *testing.T) {
	tests := []struct {
		name string
		bare bool
		want bool
	}{
		{"non-bare repo", false, false},
		{"bare repo", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &Repository{bare: tt.bare}
			if got := repo.IsBare(); got != tt.want {
				t.Errorf("IsBare() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestOpen_NonExistentPath(t *testing.T) {
	ctx := context.Background()

	_, err := Open(ctx, "/non/existent/path", OpenOptions{Bare: false})
	if err == nil {
		t.Error("Open() expected error for non-existent path")
	}
}

func TestOpen_BareRepo(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a minimal bare git repo structure
	if err := os.MkdirAll(filepath.Join(tmpDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(tmpDir, "refs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(tmpDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(ctx, tmpDir, OpenOptions{Bare: true})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if !repo.IsBare() {
		t.Error("IsBare() = false, want true")
	}
	if repo.GitDir() != tmpDir {
		t.Errorf("GitDir() = %v, want %v", repo.GitDir(), tmpDir)
	}
}

func TestOpen_RegularRepo(t *testing.T) {
	ctx := context.Background()
	tmpDir := t.TempDir()

	// Create a minimal git repo structure
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(filepath.Join(gitDir, "objects"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(gitDir, "refs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main\n"), 0644); err != nil {
		t.Fatal(err)
	}

	repo, err := Open(ctx, tmpDir, OpenOptions{Bare: false})
	if err != nil {
		t.Fatalf("Open() error = %v", err)
	}

	if repo.IsBare() {
		t.Error("IsBare() = true, want false")
	}
	if repo.Root() != tmpDir {
		t.Errorf("Root() = %v, want %v", repo.Root(), tmpDir)
	}
}

func TestGitCmd_Dir(t *testing.T) {
	cmd := newGitCmd("status")
	cmd.Dir("/custom/dir")

	if cmd.dir != "/custom/dir" {
		t.Errorf("Dir() did not set directory: got %v, want /custom/dir", cmd.dir)
	}
}

func TestGitCmd_Env(t *testing.T) {
	cmd := newGitCmd("status")
	cmd.Env("VAR1=value1", "VAR2=value2")

	if len(cmd.env) != 2 {
		t.Fatalf("Env() env length = %v, want 2", len(cmd.env))
	}
	if cmd.env[0] != "VAR1=value1" || cmd.env[1] != "VAR2=value2" {
		t.Errorf("Env() did not set env vars correctly")
	}
}

func TestGitCmd_ToExecCmd(t *testing.T) {
	ctx := context.Background()
	cmd := newGitCmd("status", "--short")
	cmd.Dir("/test/dir")
	cmd.Env("VAR1=value1")

	execCmd := cmd.toExecCmd(ctx)

	if execCmd.Dir != "/test/dir" {
		t.Errorf("toExecCmd() Dir = %v, want /test/dir", execCmd.Dir)
	}
	// Check that env contains our variable
	found := false
	for _, e := range execCmd.Env {
		if e == "VAR1=value1" {
			found = true
			break
		}
	}
	if !found {
		t.Error("toExecCmd() Env does not contain VAR1=value1")
	}
}

func TestGitCmd_Run_WithMock(t *testing.T) {
	ctx := testContext()

	t.Run("successful run", func(t *testing.T) {
		mock := &mockExecer{runErr: nil}
		cmd := newGitCmd("status")

		err := cmd.Run(ctx, mock)
		if err != nil {
			t.Errorf("Run() error = %v, want nil", err)
		}
	})

	t.Run("failed run", func(t *testing.T) {
		mock := &mockExecer{runErr: errors.New("git error")}
		cmd := newGitCmd("status")

		err := cmd.Run(ctx, mock)
		if err == nil {
			t.Error("Run() expected error")
		}
	})
}

func TestGitCmd_Output_WithMock(t *testing.T) {
	ctx := testContext()

	t.Run("successful output", func(t *testing.T) {
		mock := &mockExecer{output: []byte("abc123\n"), outputErr: nil}
		cmd := newGitCmd("rev-parse", "HEAD")

		out, err := cmd.Output(ctx, mock)
		if err != nil {
			t.Errorf("Output() error = %v, want nil", err)
		}
		if !bytes.Equal(out, []byte("abc123\n")) {
			t.Errorf("Output() = %v, want abc123", out)
		}
	})

	t.Run("failed output", func(t *testing.T) {
		mock := &mockExecer{outputErr: errors.New("git error")}
		cmd := newGitCmd("rev-parse", "HEAD")

		_, err := cmd.Output(ctx, mock)
		if err == nil {
			t.Error("Output() expected error")
		}
	})
}

func TestRepository_gitCmd_BareRepo(t *testing.T) {
	repo := &Repository{
		gitDir:  "/path/to/bare.git",
		rootDir: "/path/to/bare.git",
		bare:    true,
	}

	cmd := repo.gitCmd("status")

	// Check that GIT_DIR env is set for bare repos
	found := false
	for _, e := range cmd.env {
		if e == "GIT_DIR=/path/to/bare.git" {
			found = true
			break
		}
	}
	if !found {
		t.Error("gitCmd() for bare repo should set GIT_DIR env var")
	}
	if cmd.dir != "/path/to/bare.git" {
		t.Errorf("gitCmd() dir = %v, want /path/to/bare.git", cmd.dir)
	}
}

func TestRepository_gitCmd_NonBareRepo(t *testing.T) {
	repo := &Repository{
		gitDir:  "/path/to/repo/.git",
		rootDir: "/path/to/repo",
		bare:    false,
	}

	cmd := repo.gitCmd("status")

	// Should not set GIT_DIR env for non-bare repos
	for _, e := range cmd.env {
		if e == "GIT_DIR=/path/to/repo/.git" {
			t.Error("gitCmd() for non-bare repo should not set GIT_DIR env var")
		}
	}
	if cmd.dir != "/path/to/repo" {
		t.Errorf("gitCmd() dir = %v, want /path/to/repo", cmd.dir)
	}
}

func TestParseTreeOutput(t *testing.T) {
	tests := []struct {
		name    string
		data    []byte
		want    int
		wantErr bool
	}{
		{
			name: "single blob entry",
			data: []byte("100644 blob abc123def456789abcdef0123456789abcdef01\tpath/to/file.proto\n"),
			want: 1,
		},
		{
			name: "single tree entry",
			data: []byte("040000 tree abc123def456789abcdef0123456789abcdef01\tpath/to/dir\n"),
			want: 1,
		},
		{
			name: "multiple entries",
			data: []byte("100644 blob abc123\tfile1.proto\n100644 blob def456\tfile2.proto\n"),
			want: 2,
		},
		{
			name: "empty output",
			data: []byte(""),
			want: 0,
		},
		{
			name: "blank lines",
			data: []byte("\n\n"),
			want: 0,
		},
		{
			name: "malformed entry - no tab",
			data: []byte("100644 blob abc123 path/to/file.proto\n"),
			want: 0,
		},
		{
			name: "malformed entry - missing parts",
			data: []byte("100644 blob\tpath/to/file.proto\n"),
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entries, err := parseTreeOutput(tt.data)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTreeOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if len(entries) != tt.want {
				t.Errorf("parseTreeOutput() returned %d entries, want %d", len(entries), tt.want)
			}
		})
	}
}

func TestParseTreeOutput_EntryDetails(t *testing.T) {
	data := []byte("100644 blob abc123def456789abcdef0123456789abcdef01\tpath/to/file.proto\n")

	entries, err := parseTreeOutput(data)
	if err != nil {
		t.Fatalf("parseTreeOutput() error = %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("parseTreeOutput() returned %d entries, want 1", len(entries))
	}

	entry := entries[0]
	if entry.Mode != 0100644 {
		t.Errorf("Mode = %o, want 100644", entry.Mode)
	}
	if entry.Type != BlobType {
		t.Errorf("Type = %v, want BlobType", entry.Type)
	}
	if entry.Hash != Hash("abc123def456789abcdef0123456789abcdef01") {
		t.Errorf("Hash = %v, want abc123...", entry.Hash)
	}
	if entry.Path != "path/to/file.proto" {
		t.Errorf("Path = %v, want path/to/file.proto", entry.Path)
	}
}

// =============================================================================
// Repository Method Tests with Mocks
// =============================================================================

func TestRepository_Fetch_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		opts    FetchOptions
		mockErr error
		wantErr bool
	}{
		{
			name:    "successful fetch",
			opts:    FetchOptions{Remote: "origin"},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "fetch with options",
			opts:    FetchOptions{Remote: "origin", Depth: 1, Prune: true, Force: true},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "fetch failure",
			opts:    FetchOptions{Remote: "origin"},
			mockErr: errors.New("fetch error"),
			wantErr: true,
		},
		{
			name:    "fetch with refspecs",
			opts:    FetchOptions{Remote: "origin", RefSpecs: []Refspec{"refs/heads/main:refs/remotes/origin/main"}},
			mockErr: nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			err := repo.Fetch(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Fetch() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_Push_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		opts    PushOptions
		mockErr error
		wantErr bool
	}{
		{
			name:    "successful push",
			opts:    PushOptions{Remote: "origin"},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "push with options",
			opts:    PushOptions{Remote: "origin", Atomic: true, Force: true},
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "push failure",
			opts:    PushOptions{Remote: "origin"},
			mockErr: errors.New("push rejected"),
			wantErr: true,
		},
		{
			name:    "push with refspecs",
			opts:    PushOptions{Remote: "origin", RefSpecs: []Refspec{"refs/heads/main:refs/heads/main"}},
			mockErr: nil,
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			err := repo.Push(ctx, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("Push() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_RevExists_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		rev     string
		mockErr error
		want    bool
	}{
		{
			name:    "revision exists",
			rev:     "HEAD",
			mockErr: nil,
			want:    true,
		},
		{
			name:    "revision does not exist",
			rev:     "nonexistent",
			mockErr: errors.New("not found"),
			want:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got := repo.RevExists(ctx, tt.rev)
			if got != tt.want {
				t.Errorf("RevExists() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_RevHash_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name     string
		rev      string
		mockOut  []byte
		mockErr  error
		wantHash Hash
		wantErr  bool
	}{
		{
			name:     "successful rev-parse",
			rev:      "HEAD",
			mockOut:  []byte("abc123def456\n"),
			mockErr:  nil,
			wantHash: Hash("abc123def456"),
			wantErr:  false,
		},
		{
			name:    "rev-parse failure",
			rev:     "nonexistent",
			mockErr: errors.New("unknown revision"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.RevHash(ctx, tt.rev)
			if (err != nil) != tt.wantErr {
				t.Errorf("RevHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("RevHash() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestRepository_UpdateRef_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		ref     string
		hash    Hash
		oldHash Hash
		mockErr error
		wantErr bool
	}{
		{
			name:    "successful update",
			ref:     "refs/heads/main",
			hash:    Hash("abc123"),
			oldHash: Hash("def456"),
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "update without old hash",
			ref:     "refs/heads/main",
			hash:    Hash("abc123"),
			oldHash: Hash(""),
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "update failure",
			ref:     "refs/heads/main",
			hash:    Hash("abc123"),
			mockErr: errors.New("ref update failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			err := repo.UpdateRef(ctx, tt.ref, tt.hash, tt.oldHash)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateRef() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_GetRemoteURL_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		remote  string
		mockOut []byte
		mockErr error
		want    string
		wantErr bool
	}{
		{
			name:    "successful get",
			remote:  "origin",
			mockOut: []byte("https://github.com/user/repo.git\n"),
			mockErr: nil,
			want:    "https://github.com/user/repo.git",
			wantErr: false,
		},
		{
			name:    "get failure",
			remote:  "nonexistent",
			mockErr: errors.New("remote not found"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.GetRemoteURL(ctx, tt.remote)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRemoteURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetRemoteURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_GetRepoURL_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		mockOut []byte
		mockErr error
		want    string
		wantErr bool
	}{
		{
			name:    "https url",
			mockOut: []byte("https://github.com/user/repo.git\n"),
			mockErr: nil,
			want:    "https://github.com/user/repo",
			wantErr: false,
		},
		{
			name:    "ssh url",
			mockOut: []byte("git@github.com:user/repo.git\n"),
			mockErr: nil,
			want:    "https://github.com/user/repo",
			wantErr: false,
		},
		{
			name:    "get failure",
			mockErr: errors.New("remote not found"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.GetRepoURL(ctx)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("GetRepoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRepository_ReadTree_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		treeish Treeish
		opts    ReadTreeOptions
		mockOut []byte
		mockErr error
		want    int
		wantErr bool
	}{
		{
			name:    "read tree success",
			treeish: Treeish("HEAD"),
			opts:    ReadTreeOptions{},
			mockOut: []byte("100644 blob abc123\tfile.proto\n"),
			mockErr: nil,
			want:    1,
			wantErr: false,
		},
		{
			name:    "read tree with recurse",
			treeish: Treeish("HEAD"),
			opts:    ReadTreeOptions{Recurse: true},
			mockOut: []byte("100644 blob abc123\tdir/file.proto\n100644 blob def456\tdir/file2.proto\n"),
			mockErr: nil,
			want:    2,
			wantErr: false,
		},
		{
			name:    "read tree with paths",
			treeish: Treeish("HEAD"),
			opts:    ReadTreeOptions{Paths: []string{"dir/"}},
			mockOut: []byte("100644 blob abc123\tdir/file.proto\n"),
			mockErr: nil,
			want:    1,
			wantErr: false,
		},
		{
			name:    "read tree failure",
			treeish: Treeish("nonexistent"),
			opts:    ReadTreeOptions{},
			mockErr: errors.New("not a valid object name"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			entries, err := repo.ReadTree(ctx, tt.treeish, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && len(entries) != tt.want {
				t.Errorf("ReadTree() returned %d entries, want %d", len(entries), tt.want)
			}
		})
	}
}

func TestRepository_GetUser_WithGitConfig(t *testing.T) {
	ctx := testContext()

	t.Run("get user from git config", func(t *testing.T) {
		// Mock that returns user.name first, then user.email
		callCount := 0
		mock := &mockExecer{
			outputFunc: func() ([]byte, error) {
				callCount++
				if callCount == 1 {
					return []byte("Test User\n"), nil
				}
				return []byte("test@example.com\n"), nil
			},
		}
		repo := &Repository{
			gitDir:  "/path/to/repo/.git",
			rootDir: "/path/to/repo",
			exec:    mock,
		}

		author, err := repo.GetUser(ctx)
		if err != nil {
			t.Fatalf("GetUser() error = %v", err)
		}
		if author.Name != "Test User" {
			t.Errorf("Name = %v, want Test User", author.Name)
		}
		if author.Email != "test@example.com" {
			t.Errorf("Email = %v, want test@example.com", author.Email)
		}
		if callCount != 2 {
			t.Errorf("Expected 2 git config calls, got %d", callCount)
		}
	})

	t.Run("error when user.name not set", func(t *testing.T) {
		mock := &mockExecer{
			outputFunc: func() ([]byte, error) {
				return nil, fmt.Errorf("git config user.name failed")
			},
		}
		repo := &Repository{
			gitDir:  "/path/to/repo/.git",
			rootDir: "/path/to/repo",
			exec:    mock,
		}

		_, err := repo.GetUser(ctx)
		if err == nil {
			t.Error("GetUser() expected error when user.name not set")
		}
		if !strings.Contains(err.Error(), "get user name") {
			t.Errorf("Error message should mention 'get user name', got: %v", err)
		}
	})

	t.Run("error when user.email not set", func(t *testing.T) {
		callCount := 0
		mock := &mockExecer{
			outputFunc: func() ([]byte, error) {
				callCount++
				if callCount == 1 {
					return []byte("Test User\n"), nil
				}
				return nil, fmt.Errorf("git config user.email failed")
			},
		}
		repo := &Repository{
			gitDir:  "/path/to/repo/.git",
			rootDir: "/path/to/repo",
			exec:    mock,
		}

		_, err := repo.GetUser(ctx)
		if err == nil {
			t.Error("GetUser() expected error when user.email not set")
		}
		if !strings.Contains(err.Error(), "get user email") {
			t.Errorf("Error message should mention 'get user email', got: %v", err)
		}
	})
}

func TestGitCmd_OutputWithStdin(t *testing.T) {
	ctx := testContext()

	t.Run("successful with stdin", func(t *testing.T) {
		mock := &mockExecer{output: []byte("abc123\n"), outputErr: nil}
		cmd := newGitCmd("hash-object", "-w", "--stdin")
		stdin := bytes.NewReader([]byte("test content"))

		out, err := cmd.OutputWithStdin(ctx, mock, stdin)
		if err != nil {
			t.Errorf("OutputWithStdin() error = %v", err)
		}
		if !bytes.Equal(out, []byte("abc123\n")) {
			t.Errorf("OutputWithStdin() = %v, want abc123", out)
		}
	})
}

func TestGitCmd_RunWithStdout(t *testing.T) {
	ctx := testContext()

	t.Run("successful with stdout", func(t *testing.T) {
		mock := &mockExecer{runErr: nil}
		cmd := newGitCmd("cat-file", "blob", "abc123")
		var buf bytes.Buffer

		err := cmd.RunWithStdout(ctx, mock, &buf)
		if err != nil {
			t.Errorf("RunWithStdout() error = %v", err)
		}
	})

	t.Run("failed with stdout", func(t *testing.T) {
		mock := &mockExecer{runErr: errors.New("not found")}
		cmd := newGitCmd("cat-file", "blob", "nonexistent")
		var buf bytes.Buffer

		err := cmd.RunWithStdout(ctx, mock, &buf)
		if err == nil {
			t.Error("RunWithStdout() expected error")
		}
	})
}

func TestExecuteGitOutput(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name      string
		operation string
		args      []string
		mockOut   []byte
		mockErr   error
		want      string
		wantErr   bool
	}{
		{
			name:      "successful output",
			operation: "test-op",
			args:      []string{"some-cmd"},
			mockOut:   []byte("result\n"),
			mockErr:   nil,
			want:      "result",
			wantErr:   false,
		},
		{
			name:      "failed output",
			operation: "test-op",
			args:      []string{"some-cmd"},
			mockErr:   errors.New("command failed"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.executeGitOutput(ctx, tt.operation, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeGitOutput() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("executeGitOutput() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExecuteGitOutputToHashFromArgs(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name      string
		operation string
		args      []string
		mockOut   []byte
		mockErr   error
		wantHash  Hash
		wantErr   bool
	}{
		{
			name:      "successful hash",
			operation: "test-op",
			args:      []string{"rev-parse", "HEAD"},
			mockOut:   []byte("abc123def456\n"),
			mockErr:   nil,
			wantHash:  Hash("abc123def456"),
			wantErr:   false,
		},
		{
			name:      "failed hash",
			operation: "test-op",
			args:      []string{"rev-parse", "HEAD"},
			mockErr:   errors.New("not found"),
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.executeGitOutputToHashFromArgs(ctx, tt.operation, tt.args...)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeGitOutputToHashFromArgs() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("executeGitOutputToHashFromArgs() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestGetGitConfig(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		key     string
		mockOut []byte
		mockErr error
		want    string
		wantErr bool
	}{
		{
			name:    "successful config get",
			key:     "user.name",
			mockOut: []byte("Test User\n"),
			mockErr: nil,
			want:    "Test User",
			wantErr: false,
		},
		{
			name:    "config key not found",
			key:     "nonexistent.key",
			mockErr: errors.New("key not found"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.getGitConfig(ctx, tt.key)
			if (err != nil) != tt.wantErr {
				t.Errorf("getGitConfig() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("getGitConfig() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestRunCmdWithEnv(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name      string
		env       []string
		mockErr   error
		operation string
		wantErr   bool
	}{
		{
			name:      "successful run with env",
			env:       []string{"VAR=value"},
			mockErr:   nil,
			operation: "test-op",
			wantErr:   false,
		},
		{
			name:      "failed run with env",
			env:       []string{"VAR=value"},
			mockErr:   errors.New("command failed"),
			operation: "test-op",
			wantErr:   true,
		},
		{
			name:      "run with empty env",
			env:       []string{},
			mockErr:   nil,
			operation: "test-op",
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			cmd := newGitCmd("some-cmd")

			err := runCmdWithEnv(cmd, tt.env, ctx, mock, tt.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("runCmdWithEnv() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_WriteObject_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name     string
		opts     WriteObjectOptions
		content  string
		mockOut  []byte
		mockErr  error
		wantHash Hash
		wantErr  bool
	}{
		{
			name:     "write blob",
			opts:     WriteObjectOptions{Type: BlobType},
			content:  "test content",
			mockOut:  []byte("abc123def456\n"),
			mockErr:  nil,
			wantHash: Hash("abc123def456"),
			wantErr:  false,
		},
		{
			name:     "write with path",
			opts:     WriteObjectOptions{Type: BlobType, Path: "test.proto"},
			content:  "test content",
			mockOut:  []byte("def456abc123\n"),
			mockErr:  nil,
			wantHash: Hash("def456abc123"),
			wantErr:  false,
		},
		{
			name:    "write failure",
			opts:    WriteObjectOptions{Type: BlobType},
			content: "test content",
			mockErr: errors.New("write failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			body := bytes.NewReader([]byte(tt.content))
			got, err := repo.WriteObject(ctx, body, tt.opts)
			if (err != nil) != tt.wantErr {
				t.Errorf("WriteObject() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("WriteObject() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestRepository_ReadObject_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name    string
		objType ObjectType
		hash    Hash
		mockErr error
		wantErr bool
	}{
		{
			name:    "read blob",
			objType: BlobType,
			hash:    Hash("abc123"),
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "read tree",
			objType: TreeType,
			hash:    Hash("def456"),
			mockErr: nil,
			wantErr: false,
		},
		{
			name:    "read failure",
			objType: BlobType,
			hash:    Hash("nonexistent"),
			mockErr: errors.New("object not found"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{runErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			var buf bytes.Buffer
			err := repo.ReadObject(ctx, tt.objType, tt.hash, &buf)
			if (err != nil) != tt.wantErr {
				t.Errorf("ReadObject() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestRepository_CommitTree_WithMock(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name     string
		req      CommitTreeRequest
		mockOut  []byte
		mockErr  error
		wantHash Hash
		wantErr  bool
	}{
		{
			name: "commit without parents",
			req: CommitTreeRequest{
				Tree:    Hash("tree123"),
				Message: "Initial commit",
				Author:  Author{Name: "Test User", Email: "test@example.com"},
			},
			mockOut:  []byte("commit123\n"),
			mockErr:  nil,
			wantHash: Hash("commit123"),
			wantErr:  false,
		},
		{
			name: "commit with parent",
			req: CommitTreeRequest{
				Tree:    Hash("tree456"),
				Parents: []Hash{Hash("parent123")},
				Message: "Second commit",
				Author:  Author{Name: "Test User", Email: "test@example.com"},
			},
			mockOut:  []byte("commit456\n"),
			mockErr:  nil,
			wantHash: Hash("commit456"),
			wantErr:  false,
		},
		{
			name: "commit with multiple parents",
			req: CommitTreeRequest{
				Tree:    Hash("tree789"),
				Parents: []Hash{Hash("parent1"), Hash("parent2")},
				Message: "Merge commit",
				Author:  Author{Name: "Test User", Email: "test@example.com"},
			},
			mockOut:  []byte("mergecommit\n"),
			mockErr:  nil,
			wantHash: Hash("mergecommit"),
			wantErr:  false,
		},
		{
			name: "commit failure",
			req: CommitTreeRequest{
				Tree:    Hash("tree123"),
				Message: "Test commit",
				Author:  Author{Name: "Test User", Email: "test@example.com"},
			},
			mockErr: errors.New("commit failed"),
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			got, err := repo.CommitTree(ctx, tt.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("CommitTree() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("CommitTree() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestExecuteGitOutputToHash(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name      string
		env       []string
		mockOut   []byte
		mockErr   error
		operation string
		wantHash  Hash
		wantErr   bool
	}{
		{
			name:      "successful with env",
			env:       []string{"GIT_INDEX_FILE=/tmp/index"},
			mockOut:   []byte("treehash123\n"),
			mockErr:   nil,
			operation: "write-tree",
			wantHash:  Hash("treehash123"),
			wantErr:   false,
		},
		{
			name:      "successful without env",
			env:       nil,
			mockOut:   []byte("hash456\n"),
			mockErr:   nil,
			operation: "test-op",
			wantHash:  Hash("hash456"),
			wantErr:   false,
		},
		{
			name:      "failure",
			env:       nil,
			mockErr:   errors.New("command failed"),
			operation: "test-op",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			cmd := repo.gitCmd("write-tree")
			got, err := repo.executeGitOutputToHash(ctx, cmd, tt.env, tt.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeGitOutputToHash() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("executeGitOutputToHash() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestExecuteGitOutputToHashWithStdin(t *testing.T) {
	ctx := testContext()

	tests := []struct {
		name      string
		stdin     string
		mockOut   []byte
		mockErr   error
		operation string
		wantHash  Hash
		wantErr   bool
	}{
		{
			name:      "successful with stdin",
			stdin:     "test content",
			mockOut:   []byte("blobhash123\n"),
			mockErr:   nil,
			operation: "hash-object",
			wantHash:  Hash("blobhash123"),
			wantErr:   false,
		},
		{
			name:      "failure with stdin",
			stdin:     "test content",
			mockErr:   errors.New("hash-object failed"),
			operation: "hash-object",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mock := &mockExecer{output: tt.mockOut, outputErr: tt.mockErr}
			repo := &Repository{
				gitDir:  "/path/to/repo/.git",
				rootDir: "/path/to/repo",
				exec:    mock,
			}

			cmd := repo.gitCmd("hash-object", "-w", "--stdin")
			stdin := bytes.NewReader([]byte(tt.stdin))
			got, err := repo.executeGitOutputToHashWithStdin(ctx, cmd, stdin, tt.operation)
			if (err != nil) != tt.wantErr {
				t.Errorf("executeGitOutputToHashWithStdin() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.wantHash {
				t.Errorf("executeGitOutputToHashWithStdin() = %v, want %v", got, tt.wantHash)
			}
		})
	}
}

func TestClone(t *testing.T) {
	ctx := testContext()

	t.Run("clone options building", func(t *testing.T) {
		// We can't fully test Clone without a real git repo,
		// but we can test that it builds the correct command args
		// by testing with a mock that returns an error
		mock := &mockExecer{runErr: errors.New("clone test")}
		ctx := WithExecer(ctx, mock)

		_, err := Clone(ctx, "https://example.com/repo.git", "/tmp/test", CloneOptions{
			Bare:   true,
			NoTags: true,
			Depth:  1,
		})
		if err == nil {
			t.Error("Clone() expected error from mock")
		}
	})
}

func TestTreeEntry_Fields(t *testing.T) {
	entry := TreeEntry{
		Mode: 0100644,
		Type: BlobType,
		Hash: Hash("abc123"),
		Path: "test/file.proto",
	}

	if entry.Mode != 0100644 {
		t.Errorf("Mode = %o, want 100644", entry.Mode)
	}
	if entry.Type != BlobType {
		t.Errorf("Type = %v, want BlobType", entry.Type)
	}
	if entry.Hash != Hash("abc123") {
		t.Errorf("Hash = %v, want abc123", entry.Hash)
	}
	if entry.Path != "test/file.proto" {
		t.Errorf("Path = %v, want test/file.proto", entry.Path)
	}
}

func TestAuthor_Fields(t *testing.T) {
	author := Author{
		Name:  "Test User",
		Email: "test@example.com",
	}

	if author.Name != "Test User" {
		t.Errorf("Name = %v, want Test User", author.Name)
	}
	if author.Email != "test@example.com" {
		t.Errorf("Email = %v, want test@example.com", author.Email)
	}
}
