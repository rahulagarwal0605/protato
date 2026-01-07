// Package git provides Git operations abstraction layer.
package git

import (
	"context"
	"fmt"
	"os/exec"
)

// Hash represents a Git commit/tree/blob hash.
type Hash string

// String returns the hash as a string.
func (h Hash) String() string {
	return string(h)
}

// Short returns the first 7 characters of the hash.
func (h Hash) Short() string {
	if len(h) > 7 {
		return string(h[:7])
	}
	return string(h)
}

// Treeish represents a commit-ish reference.
type Treeish string

// String returns the treeish as a string.
func (t Treeish) String() string {
	return string(t)
}

// Refspec represents a Git refspec (src:dst).
type Refspec string

// ObjectType represents the type of Git object.
type ObjectType int

const (
	BlobType ObjectType = iota
	TreeType
	CommitType
	TagType
)

// String returns the object type as a string.
func (t ObjectType) String() string {
	switch t {
	case BlobType:
		return "blob"
	case TreeType:
		return "tree"
	case CommitType:
		return "commit"
	case TagType:
		return "tag"
	default:
		return "unknown"
	}
}

// ParseObjectType parses a string into an ObjectType.
func ParseObjectType(s string) (ObjectType, error) {
	switch s {
	case "blob":
		return BlobType, nil
	case "tree":
		return TreeType, nil
	case "commit":
		return CommitType, nil
	case "tag":
		return TagType, nil
	default:
		return 0, fmt.Errorf("unknown object type: %s", s)
	}
}

// TreeEntry represents an entry in a Git tree.
type TreeEntry struct {
	Mode uint32     // File mode (100644, 040000, etc.)
	Type ObjectType // Object type
	Hash Hash       // Object hash
	Path string     // File path
}

// Author represents a Git author/committer.
type Author struct {
	Name  string
	Email string
}

// Execer is an interface for executing commands.
type Execer interface {
	Run(cmd *exec.Cmd) error
	Output(cmd *exec.Cmd) ([]byte, error)
}

// DefaultExecer is the default command executor.
type DefaultExecer struct{}

// Run executes a command.
func (e *DefaultExecer) Run(cmd *exec.Cmd) error {
	return cmd.Run()
}

// Output executes a command and returns its output.
func (e *DefaultExecer) Output(cmd *exec.Cmd) ([]byte, error) {
	return cmd.Output()
}

// CloneOptions contains options for cloning a repository.
type CloneOptions struct {
	Bare   bool // Clone as bare repository
	NoTags bool // Don't clone tags
	Depth  int  // Shallow clone depth
}

// OpenOptions contains options for opening a repository.
type OpenOptions struct {
	Bare bool // Open as bare repository
}

// FetchOptions contains options for fetching.
type FetchOptions struct {
	Remote   string    // Remote name
	RefSpecs []Refspec // Refspecs to fetch
	Depth    int       // Fetch depth
	Prune    bool      // Prune remote tracking refs
}

// PushOptions contains options for pushing.
type PushOptions struct {
	Remote   string    // Remote name
	RefSpecs []Refspec // Refspecs to push
	Atomic   bool      // Atomic push
	Force    bool      // Force push
}

// ReadTreeOptions contains options for reading a tree.
type ReadTreeOptions struct {
	Recurse bool     // Recurse into subtrees
	Paths   []string // Paths to read
}

// WriteObjectOptions contains options for writing an object.
type WriteObjectOptions struct {
	Type ObjectType // Object type
	Path string     // Path hint for blob
}

// UpdateTreeRequest contains parameters for updating a tree.
type UpdateTreeRequest struct {
	Tree    Hash         // Base tree
	Upserts []TreeUpsert // Files to add/update
	Deletes []string     // Files to delete
}

// TreeUpsert represents a file to add or update in a tree.
type TreeUpsert struct {
	Path string // File path
	Blob Hash   // Blob hash
	Mode uint32 // File mode
}

// CommitTreeRequest contains parameters for creating a commit.
type CommitTreeRequest struct {
	Tree    Hash   // Tree hash
	Parents []Hash // Parent commits
	Message string // Commit message
	Author  Author // Author/committer
}

// RevParseOptions contains options for git rev-parse.
type RevParseOptions struct {
	Verify bool // Verify the object exists
}

// contextKey is used for context values.
type contextKey string

// execerContextKey is the context key for the execer.
const execerContextKey contextKey = "execer"

// WithExecer returns a context with the given execer.
func WithExecer(ctx context.Context, e Execer) context.Context {
	return context.WithValue(ctx, execerContextKey, e)
}

// GetExecer returns the execer from the context.
func GetExecer(ctx context.Context) Execer {
	if e, ok := ctx.Value(execerContextKey).(Execer); ok {
		return e
	}
	return &DefaultExecer{}
}
