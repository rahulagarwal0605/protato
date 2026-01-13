package utils

import (
	"strings"
)

// HasServicePrefix checks if a path has the given service prefix.
func HasServicePrefix(path, servicePrefix string) bool {
	return servicePrefix != "" && strings.HasPrefix(path, servicePrefix+"/")
}

// TrimServicePrefix removes the service prefix from a path.
func TrimServicePrefix(path, servicePrefix string) string {
	return strings.TrimPrefix(path, servicePrefix+"/")
}

// ExtractServicePrefixFromProject extracts the service prefix from a project path.
// Returns empty string if no prefix found.
func ExtractServicePrefixFromProject(projectPath string) string {
	if idx := strings.Index(projectPath, "/"); idx > 0 {
		return projectPath[:idx]
	}
	return ""
}

// SplitContentToLines splits content into lines.
func SplitContentToLines(content []byte) []string {
	return strings.Split(string(content), "\n")
}

// JoinLines joins lines back into content.
func JoinLines(lines []string) []byte {
	return []byte(strings.Join(lines, "\n"))
}

// TrimOutputToString converts command output to a trimmed string.
func TrimOutputToString(out []byte) string {
	return strings.TrimSpace(string(out))
}

// ReplaceStringInLine replaces a string in a line (first occurrence).
func ReplaceStringInLine(line, oldStr, newStr string) string {
	return strings.Replace(line, oldStr, newStr, 1)
}

// BuildServicePrefixedPath builds a path with service prefix.
func BuildServicePrefixedPath(service, projectPath string) string {
	return service + "/" + projectPath
}

// ParseCommaSeparated parses a comma-separated string into a slice of trimmed, non-empty strings.
func ParseCommaSeparated(input string) []string {
	var result []string
	for _, p := range strings.Split(input, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// ExtractLiteralPaths filters a slice of patterns to return only literal paths (no glob patterns).
// A pattern is considered a glob if it contains '*' or '?' characters.
func ExtractLiteralPaths(patterns []string) []string {
	var literalPaths []string
	for _, pattern := range patterns {
		// Check if pattern contains glob characters
		if !strings.Contains(pattern, "*") && !strings.Contains(pattern, "?") {
			literalPaths = append(literalPaths, pattern)
		}
	}
	return literalPaths
}

// ContainsAny checks if a string contains any of the given substrings.
func ContainsAny(s string, substrings ...string) bool {
	for _, substr := range substrings {
		if strings.Contains(s, substr) {
			return true
		}
	}
	return false
}

// HasAnyPrefix checks if a string starts with any of the given prefixes (with "/" separator) or equals any prefix exactly.
// Example: HasAnyPrefix("service/path", []string{"service", "other"}) -> true
//          HasAnyPrefix("service", []string{"service", "other"}) -> true
//          HasAnyPrefix("other/path", []string{"service"}) -> false
func HasAnyPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix+"/") || s == prefix {
			return true
		}
	}
	return false
}
