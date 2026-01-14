package utils

import (
	"testing"
)

func TestHasServicePrefix(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		servicePrefix string
		want          bool
	}{
		{
			name:          "has prefix",
			path:          "service/path/to/file",
			servicePrefix: "service",
			want:          true,
		},
		{
			name:          "no prefix",
			path:          "other/path/to/file",
			servicePrefix: "service",
			want:          false,
		},
		{
			name:          "empty prefix",
			path:          "service/path/to/file",
			servicePrefix: "",
			want:          false,
		},
		{
			name:          "exact match",
			path:          "service",
			servicePrefix: "service",
			want:          false, // Must have "/" after prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasServicePrefix(tt.path, tt.servicePrefix)
			if got != tt.want {
				t.Errorf("HasServicePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimServicePrefix(t *testing.T) {
	tests := []struct {
		name          string
		path          string
		servicePrefix string
		want          string
	}{
		{
			name:          "trim prefix",
			path:          "service/path/to/file",
			servicePrefix: "service",
			want:          "path/to/file",
		},
		{
			name:          "no prefix to trim",
			path:          "other/path/to/file",
			servicePrefix: "service",
			want:          "other/path/to/file",
		},
		{
			name:          "empty prefix",
			path:          "service/path/to/file",
			servicePrefix: "",
			want:          "service/path/to/file",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimServicePrefix(tt.path, tt.servicePrefix)
			if got != tt.want {
				t.Errorf("TrimServicePrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestExtractServicePrefixFromProject(t *testing.T) {
	tests := []struct {
		name        string
		projectPath string
		want        string
	}{
		{
			name:        "extract prefix",
			projectPath: "service/project",
			want:        "service",
		},
		{
			name:        "no prefix",
			projectPath: "project",
			want:        "",
		},
		{
			name:        "nested path",
			projectPath: "service/project/v1",
			want:        "service",
		},
		{
			name:        "empty path",
			projectPath: "",
			want:        "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractServicePrefixFromProject(tt.projectPath)
			if got != tt.want {
				t.Errorf("ExtractServicePrefixFromProject() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildServicePrefixedPath(t *testing.T) {
	tests := []struct {
		name        string
		service     string
		projectPath string
		want        string
	}{
		{
			name:        "build prefixed path",
			service:     "service",
			projectPath: "project",
			want:        "service/project",
		},
		{
			name:        "nested project path",
			service:     "service",
			projectPath: "project/v1",
			want:        "service/project/v1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildServicePrefixedPath(tt.service, tt.projectPath)
			if got != tt.want {
				t.Errorf("BuildServicePrefixedPath() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestParseCommaSeparated(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  []string
	}{
		{
			name:  "parse comma separated",
			input: "a, b, c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "with empty items",
			input: "a, , b, c",
			want:  []string{"a", "b", "c"},
		},
		{
			name:  "single item",
			input: "a",
			want:  []string{"a"},
		},
		{
			name:  "empty string",
			input: "",
			want:  []string{},
		},
		{
			name:  "with spaces",
			input: "  a  ,  b  ,  c  ",
			want:  []string{"a", "b", "c"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ParseCommaSeparated(tt.input)
			if len(got) != len(tt.want) {
				t.Errorf("ParseCommaSeparated() length = %v, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("ParseCommaSeparated()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestExtractLiteralPaths(t *testing.T) {
	tests := []struct {
		name     string
		patterns []string
		want     []string
	}{
		{
			name:     "extract literals",
			patterns: []string{"team/service", "team/*", "other/service"},
			want:     []string{"team/service", "other/service"},
		},
		{
			name:     "all globs",
			patterns: []string{"team/*", "**/*.proto"},
			want:     []string{},
		},
		{
			name:     "all literals",
			patterns: []string{"team/service", "other/service"},
			want:     []string{"team/service", "other/service"},
		},
		{
			name:     "with question mark",
			patterns: []string{"team/service", "team/serv?ce"},
			want:     []string{"team/service"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExtractLiteralPaths(tt.patterns)
			if len(got) != len(tt.want) {
				t.Errorf("ExtractLiteralPaths() length = %v, want %v", len(got), len(tt.want))
			}
			gotMap := make(map[string]bool)
			for _, p := range got {
				gotMap[p] = true
			}
			for _, w := range tt.want {
				if !gotMap[w] {
					t.Errorf("ExtractLiteralPaths() missing: %s", w)
				}
			}
		})
	}
}

func TestContainsAny(t *testing.T) {
	tests := []struct {
		name       string
		s          string
		substrings []string
		want       bool
	}{
		{
			name:       "contains one",
			s:          "hello world",
			substrings: []string{"world", "foo"},
			want:       true,
		},
		{
			name:       "contains none",
			s:          "hello world",
			substrings: []string{"foo", "bar"},
			want:       false,
		},
		{
			name:       "empty substrings",
			s:          "hello world",
			substrings: []string{},
			want:       false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContainsAny(tt.s, tt.substrings...)
			if got != tt.want {
				t.Errorf("ContainsAny() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestHasAnyPrefix(t *testing.T) {
	tests := []struct {
		name     string
		s        string
		prefixes []string
		want     bool
	}{
		{
			name:     "has prefix",
			s:        "service/path",
			prefixes: []string{"service", "other"},
			want:     true,
		},
		{
			name:     "exact match",
			s:        "service",
			prefixes: []string{"service", "other"},
			want:     true,
		},
		{
			name:     "no match",
			s:        "other/path",
			prefixes: []string{"service"},
			want:     false,
		},
		{
			name:     "empty prefixes",
			s:        "service/path",
			prefixes: []string{},
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := HasAnyPrefix(tt.s, tt.prefixes)
			if got != tt.want {
				t.Errorf("HasAnyPrefix() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSplitContentToLines(t *testing.T) {
	tests := []struct {
		name    string
		content []byte
		want    []string
	}{
		{
			name:    "split into lines",
			content: []byte("line1\nline2\nline3"),
			want:    []string{"line1", "line2", "line3"},
		},
		{
			name:    "single line",
			content: []byte("single line"),
			want:    []string{"single line"},
		},
		{
			name:    "empty content",
			content: []byte(""),
			want:    []string{""},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := SplitContentToLines(tt.content)
			if len(got) != len(tt.want) {
				t.Errorf("SplitContentToLines() length = %v, want %v", len(got), len(tt.want))
			}
			for i := range got {
				if got[i] != tt.want[i] {
					t.Errorf("SplitContentToLines()[%d] = %v, want %v", i, got[i], tt.want[i])
				}
			}
		})
	}
}

func TestJoinLines(t *testing.T) {
	tests := []struct {
		name  string
		lines []string
		want  []byte
	}{
		{
			name:  "join lines",
			lines: []string{"line1", "line2", "line3"},
			want:  []byte("line1\nline2\nline3"),
		},
		{
			name:  "single line",
			lines: []string{"single line"},
			want:  []byte("single line"),
		},
		{
			name:  "empty lines",
			lines: []string{},
			want:  []byte(""),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := JoinLines(tt.lines)
			if string(got) != string(tt.want) {
				t.Errorf("JoinLines() = %v, want %v", string(got), string(tt.want))
			}
		})
	}
}

func TestReplaceStringInLine(t *testing.T) {
	tests := []struct {
		name    string
		line    string
		oldStr  string
		newStr  string
		want    string
	}{
		{
			name:    "replace first occurrence",
			line:    "import proto/common",
			oldStr:  "proto/",
			newStr:  "service/",
			want:    "import service/common",
		},
		{
			name:    "no match",
			line:    "import other/common",
			oldStr:  "proto/",
			newStr:  "service/",
			want:    "import other/common",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ReplaceStringInLine(tt.line, tt.oldStr, tt.newStr)
			if got != tt.want {
				t.Errorf("ReplaceStringInLine() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestTrimOutputToString(t *testing.T) {
	tests := []struct {
		name string
		out  []byte
		want string
	}{
		{
			name: "trim newline",
			out:  []byte("output\n"),
			want: "output",
		},
		{
			name: "trim spaces and newline",
			out:  []byte("  output  \n"),
			want: "output",
		},
		{
			name: "trim multiple newlines",
			out:  []byte("output\n\n"),
			want: "output",
		},
		{
			name: "no trimming needed",
			out:  []byte("output"),
			want: "output",
		},
		{
			name: "empty output",
			out:  []byte(""),
			want: "",
		},
		{
			name: "only whitespace",
			out:  []byte("   \n  \t  "),
			want: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := TrimOutputToString(tt.out)
			if got != tt.want {
				t.Errorf("TrimOutputToString() = %q, want %q", got, tt.want)
			}
		})
	}
}
