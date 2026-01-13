package git

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/logger"
	"github.com/rahulagarwal0605/protato/internal/utils"
)

// Repository represents a Git repository.
type Repository struct {
	gitDir  string // .git directory
	bare    bool   // Bare repository flag
	rootDir string // Working directory
	exec    Execer // Command executor
}

// Clone clones a repository.
func Clone(ctx context.Context, url, path string, opts CloneOptions) (*Repository, error) {
	args := []string{"clone"}
	if opts.Bare {
		args = append(args, "--bare")
	}
	if opts.NoTags {
		args = append(args, "--no-tags")
	}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	args = append(args, url, path)

	cmd := newGitCmd(args...)
	if err := cmd.Run(ctx, GetExecer(ctx)); err != nil {
		return nil, fmt.Errorf("clone: %w", err)
	}

	return Open(ctx, path, OpenOptions{Bare: opts.Bare})
}

// Open opens an existing repository.
func Open(ctx context.Context, path string, opts OpenOptions) (*Repository, error) {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("abs path: %w", err)
	}

	repo := &Repository{
		exec: GetExecer(ctx),
		bare: opts.Bare,
	}

	if opts.Bare {
		repo.gitDir = absPath
		repo.rootDir = absPath
	} else {
		repo.rootDir = absPath
		repo.gitDir = filepath.Join(absPath, ".git")
	}

	// Verify it's a Git repository
	if _, err := os.Stat(repo.gitDir); os.IsNotExist(err) {
		// For non-bare repos, check if .git is a file (worktree)
		if !opts.Bare {
			if _, err := os.Stat(filepath.Join(absPath, ".git")); os.IsNotExist(err) {
				return nil, errNotGitRepository(path)
			}
		} else {
			return nil, errNotGitRepository(path)
		}
	}

	return repo, nil
}

// Root returns the repository root directory.
func (r *Repository) Root() string {
	return r.rootDir
}

// GitDir returns the .git directory.
func (r *Repository) GitDir() string {
	return r.gitDir
}

// IsBare returns true if this is a bare repository.
func (r *Repository) IsBare() bool {
	return r.bare
}

// gitCmd creates a new Git command.
func (r *Repository) gitCmd(args ...string) *gitCmd {
	cmd := newGitCmd(args...)
	if r.bare {
		cmd.env = append(cmd.env, "GIT_DIR="+r.gitDir)
	} else {
		cmd.dir = r.rootDir
	}
	return cmd
}

// Fetch fetches from a remote.
func (r *Repository) Fetch(ctx context.Context, opts FetchOptions) error {
	args := []string{"fetch"}
	if opts.Depth > 0 {
		args = append(args, "--depth", strconv.Itoa(opts.Depth))
	}
	if opts.Prune {
		args = append(args, "--prune")
	}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Remote != "" {
		args = append(args, opts.Remote)
	}
	args = appendRefspecs(args, opts.RefSpecs)

	return r.gitCmd(args...).Run(ctx, r.exec)
}

// Push pushes to a remote.
func (r *Repository) Push(ctx context.Context, opts PushOptions) error {
	args := []string{"push"}
	if opts.Atomic {
		args = append(args, "--atomic")
	}
	if opts.Force {
		args = append(args, "--force")
	}
	if opts.Remote != "" {
		args = append(args, opts.Remote)
	}
	args = appendRefspecs(args, opts.RefSpecs)

	return r.gitCmd(args...).Run(ctx, r.exec)
}

// trimOutputToHash converts command output to a Hash.
func trimOutputToHash(out []byte) Hash {
	return Hash(utils.TrimOutputToString(out))
}

// appendRefspecs appends refspecs to args slice.
func appendRefspecs(args []string, refspecs []Refspec) []string {
	for _, refspec := range refspecs {
		args = append(args, string(refspec))
	}
	return args
}

// appendEnvToCmd appends environment variables to a git command.
func appendEnvToCmd(cmd *gitCmd, env []string) {
	cmd.env = append(cmd.env, env...)
}

// errNotGitRepository returns an error for invalid git repository.
func errNotGitRepository(path string) error {
	return fmt.Errorf("not a git repository: %s", path)
}

// logGitCommand logs a git command execution.
func (c *gitCmd) logGitCommand(ctx context.Context, msg string) {
	logger.Log(ctx).Debug().
		Strs("args", c.args).
		Str("dir", c.dir).
		Msg(msg)
}

// executeGitOutput executes a git command and returns trimmed string output with error handling.
func (r *Repository) executeGitOutput(ctx context.Context, operation string, args ...string) (string, error) {
	out, err := r.gitCmd(args...).Output(ctx, r.exec)
	if err != nil {
		return "", fmt.Errorf("%s: %w", operation, err)
	}
	return utils.TrimOutputToString(out), nil
}

// executeGitOutputToHashFromArgs executes a git command and returns a Hash.
func (r *Repository) executeGitOutputToHashFromArgs(ctx context.Context, operation string, args ...string) (Hash, error) {
	str, err := r.executeGitOutput(ctx, operation, args...)
	if err != nil {
		return "", err
	}
	return Hash(str), nil
}

// executeGitOutputToHashWithStdin executes a git command with stdin and returns a Hash.
func (r *Repository) executeGitOutputToHashWithStdin(ctx context.Context, cmd *gitCmd, stdin io.Reader, operation string) (Hash, error) {
	out, err := cmd.OutputWithStdin(ctx, r.exec, stdin)
	if err != nil {
		return "", fmt.Errorf("%s: %w", operation, err)
	}
	return trimOutputToHash(out), nil
}

// executeGitOutputToHash executes a git command with optional env vars and returns a Hash.
func (r *Repository) executeGitOutputToHash(ctx context.Context, cmd *gitCmd, env []string, operation string) (Hash, error) {
	if len(env) > 0 {
		appendEnvToCmd(cmd, env)
	}
	out, err := cmd.Output(ctx, r.exec)
	if err != nil {
		return "", fmt.Errorf("%s: %w", operation, err)
	}
	return trimOutputToHash(out), nil
}

// getGitConfig gets a git config value.
func (r *Repository) getGitConfig(ctx context.Context, key string) (string, error) {
	return r.executeGitOutput(ctx, fmt.Sprintf("config %s", key), "config", key)
}

// runCmdWithEnv runs a git command with environment variables and handles errors.
func runCmdWithEnv(cmd *gitCmd, env []string, ctx context.Context, exec Execer, operation string) error {
	appendEnvToCmd(cmd, env)
	if err := cmd.Run(ctx, exec); err != nil {
		return fmt.Errorf("%s: %w", operation, err)
	}
	return nil
}

// RevHash resolves a revision to a hash.
func (r *Repository) RevHash(ctx context.Context, rev string) (Hash, error) {
	return r.executeGitOutputToHashFromArgs(ctx, fmt.Sprintf("rev-parse %s", rev), "rev-parse", rev)
}

// RevExists checks if a revision exists.
func (r *Repository) RevExists(ctx context.Context, rev string) bool {
	err := r.gitCmd("rev-parse", "--verify", rev+"^{commit}").Run(ctx, r.exec)
	return err == nil
}

// ReadTree reads a tree's contents.
func (r *Repository) ReadTree(ctx context.Context, treeish Treeish, opts ReadTreeOptions) ([]TreeEntry, error) {
	args := []string{"ls-tree"}
	if opts.Recurse {
		args = append(args, "-r")
	}
	args = append(args, string(treeish))
	if len(opts.Paths) > 0 {
		args = append(args, "--")
		args = append(args, opts.Paths...)
	}

	out, err := r.gitCmd(args...).Output(ctx, r.exec)
	if err != nil {
		return nil, fmt.Errorf("ls-tree: %w", err)
	}

	return parseTreeOutput(out)
}

// parseTreeOutput parses the output of git ls-tree.
func parseTreeOutput(data []byte) ([]TreeEntry, error) {
	var entries []TreeEntry
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}

		// Format: <mode> <type> <hash>\t<path>
		parts := strings.SplitN(line, "\t", 2)
		if len(parts) != 2 {
			continue
		}

		meta := strings.Fields(parts[0])
		if len(meta) != 3 {
			continue
		}

		mode, err := strconv.ParseUint(meta[0], 8, 32)
		if err != nil {
			continue
		}

		objType, err := ParseObjectType(meta[1])
		if err != nil {
			continue
		}

		entries = append(entries, TreeEntry{
			Mode: uint32(mode),
			Type: objType,
			Hash: Hash(meta[2]),
			Path: parts[1],
		})
	}

	return entries, scanner.Err()
}

// WriteObject writes an object to the store.
func (r *Repository) WriteObject(ctx context.Context, body io.Reader, opts WriteObjectOptions) (Hash, error) {
	args := []string{"hash-object", "-w", "--stdin"}
	if opts.Type != BlobType {
		args = append(args, "-t", opts.Type.String())
	}
	if opts.Path != "" {
		args = append(args, "--path="+opts.Path)
	}

	cmd := r.gitCmd(args...)
	return r.executeGitOutputToHashWithStdin(ctx, cmd, body, "hash-object")
}

// ReadObject reads an object from the store.
func (r *Repository) ReadObject(ctx context.Context, objType ObjectType, hash Hash, writer io.Writer) error {
	cmd := r.gitCmd("cat-file", objType.String(), hash.String())
	return cmd.RunWithStdout(ctx, r.exec, writer)
}

// UpdateTree updates a tree with the given changes.
func (r *Repository) UpdateTree(ctx context.Context, req UpdateTreeRequest) (Hash, error) {
	// Create temporary index file
	indexFile, err := os.CreateTemp("", "protato-index-*")
	if err != nil {
		return "", fmt.Errorf("create temp index: %w", err)
	}
	indexPath := indexFile.Name()
	indexFile.Close()
	defer os.Remove(indexPath)

	env := []string{"GIT_INDEX_FILE=" + indexPath}

	// Read current tree into index
	if req.Tree != "" {
		cmd := r.gitCmd("read-tree", req.Tree.String())
		if err := runCmdWithEnv(cmd, env, ctx, r.exec, "read-tree"); err != nil {
			return "", err
		}
	}

	// Apply upserts
	for _, upsert := range req.Upserts {
		cmd := r.gitCmd("update-index", "--add", "--cacheinfo", fmt.Sprintf("%o,%s,%s", upsert.Mode, upsert.Blob, upsert.Path))
		if err := runCmdWithEnv(cmd, env, ctx, r.exec, "update-index add"); err != nil {
			return "", err
		}
	}

	// Apply deletes
	for _, del := range req.Deletes {
		cmd := r.gitCmd("update-index", "--remove", del)
		if err := runCmdWithEnv(cmd, env, ctx, r.exec, "update-index remove"); err != nil {
			return "", err
		}
	}

	// Write tree
	cmd := r.gitCmd("write-tree")
	return r.executeGitOutputToHash(ctx, cmd, env, "write-tree")
}

// CommitTree creates a new commit.
func (r *Repository) CommitTree(ctx context.Context, req CommitTreeRequest) (Hash, error) {
	args := []string{"commit-tree", req.Tree.String()}

	for _, parent := range req.Parents {
		args = append(args, "-p", parent.String())
	}

	args = append(args, "-m", req.Message)

	cmd := r.gitCmd(args...)
	env := []string{
		"GIT_AUTHOR_NAME=" + req.Author.Name,
		"GIT_AUTHOR_EMAIL=" + req.Author.Email,
		"GIT_COMMITTER_NAME=" + req.Author.Name,
		"GIT_COMMITTER_EMAIL=" + req.Author.Email,
	}
	return r.executeGitOutputToHash(ctx, cmd, env, "commit-tree")
}

// UpdateRef updates a reference.
func (r *Repository) UpdateRef(ctx context.Context, ref string, hash Hash, oldHash Hash) error {
	args := []string{"update-ref", ref, hash.String()}
	if oldHash != "" {
		args = append(args, oldHash.String())
	}
	return r.gitCmd(args...).Run(ctx, r.exec)
}

// GetRemoteURL gets the URL of a remote.
func (r *Repository) GetRemoteURL(ctx context.Context, remote string) (string, error) {
	return r.executeGitOutput(ctx, "get remote url", "remote", "get-url", remote)
}

// GetUser gets the current Git user (name and email).
// Checks environment variables first, then falls back to git config.
func (r *Repository) GetUser(ctx context.Context) (Author, error) {
	var author Author

	// Check GitHub Actions environment variables first
	if name := os.Getenv("GITHUB_ACTOR"); name != "" {
		author.Name = name
		email := os.Getenv("GITHUB_ACTOR_EMAIL")
		if email == "" {
			return author, fmt.Errorf("GITHUB_ACTOR_EMAIL environment variable not set")
		}
		author.Email = email
		return author, nil
	}

	// Fall back to git config
	name, err := r.getGitConfig(ctx, "user.name")
	if err != nil {
		return author, fmt.Errorf("get user name: %w", err)
	}
	author.Name = name

	email, err := r.getGitConfig(ctx, "user.email")
	if err != nil {
		return author, fmt.Errorf("get user email: %w", err)
	}
	author.Email = email

	return author, nil
}


// gitCmd is a helper for executing git commands.
type gitCmd struct {
	args []string
	dir  string
	env  []string
}

// newGitCmd creates a new git command.
func newGitCmd(args ...string) *gitCmd {
	return &gitCmd{
		args: args,
	}
}

// Dir sets the working directory.
func (c *gitCmd) Dir(dir string) *gitCmd {
	c.dir = dir
	return c
}

// Env adds environment variables.
func (c *gitCmd) Env(env ...string) *gitCmd {
	c.env = append(c.env, env...)
	return c
}

// toExecCmd converts to an exec.Cmd.
func (c *gitCmd) toExecCmd(ctx context.Context) *exec.Cmd {
	cmd := exec.CommandContext(ctx, "git", c.args...)
	if c.dir != "" {
		cmd.Dir = c.dir
	}
	if len(c.env) > 0 {
		cmd.Env = append(os.Environ(), c.env...)
	}
	return cmd
}

// Run executes the command.
func (c *gitCmd) Run(ctx context.Context, e Execer) error {
	c.logGitCommand(ctx, "Executing git command")
	return e.Run(c.toExecCmd(ctx))
}

// Output executes the command and returns its output.
func (c *gitCmd) Output(ctx context.Context, e Execer) ([]byte, error) {
	c.logGitCommand(ctx, "Executing git command")
	return e.Output(c.toExecCmd(ctx))
}

// OutputWithStdin executes the command with stdin and returns its output.
func (c *gitCmd) OutputWithStdin(ctx context.Context, e Execer, stdin io.Reader) ([]byte, error) {
	cmd := c.toExecCmd(ctx)
	cmd.Stdin = stdin
	c.logGitCommand(ctx, "Executing git command with stdin")
	return e.Output(cmd)
}

// RunWithStdout executes the command and writes stdout to the writer.
func (c *gitCmd) RunWithStdout(ctx context.Context, e Execer, stdout io.Writer) error {
	cmd := c.toExecCmd(ctx)
	cmd.Stdout = stdout
	c.logGitCommand(ctx, "Executing git command with stdout")
	return e.Run(cmd)
}

// GetRepoURL returns the normalized remote URL for the repository.
func (r *Repository) GetRepoURL(ctx context.Context) (string, error) {
	repoURL, err := r.GetRemoteURL(ctx, "origin")
	if err != nil {
		return "", fmt.Errorf("get remote URL: %w", err)
	}
	return utils.NormalizeGitURL(repoURL), nil
}
