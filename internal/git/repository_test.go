package git

import (
	"testing"
)

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
}

func TestAppendEnvToCmd_EmptyEnv(t *testing.T) {
	cmd := &gitCmd{
		args: []string{"git", "command"},
		env:  []string{"VAR1=value1"},
	}

	appendEnvToCmd(cmd, []string{})

	if len(cmd.env) != 1 {
		t.Errorf("appendEnvToCmd() env length = %v, want 1", len(cmd.env))
	}
	if cmd.env[0] != "VAR1=value1" {
		t.Errorf("appendEnvToCmd() env[0] = %v, want VAR1=value1", cmd.env[0])
	}
}

func TestErrNotGitRepository(t *testing.T) {
	path := "/path/to/non-git/dir"
	err := errNotGitRepository(path)

	if err == nil {
		t.Fatal("errNotGitRepository() returned nil")
	}

	errMsg := err.Error()
	if errMsg == "" {
		t.Error("errNotGitRepository() error message is empty")
	}
	if errMsg != "not a git repository: /path/to/non-git/dir" {
		t.Errorf("errNotGitRepository() error = %v, want 'not a git repository: /path/to/non-git/dir'", errMsg)
	}
}
