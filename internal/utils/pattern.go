package utils

import (
	"github.com/bmatcuk/doublestar/v4"
)

// MatchPattern checks if a path matches a glob pattern (with or without trailing slash).
// This handles directory patterns by checking both the path and path with trailing slash.
func MatchPattern(pattern, projectPath string) bool {
	// Check pattern against path
	if match, _ := doublestar.Match(pattern, projectPath); match {
		return true
	}
	// Also check with trailing slash to match directory patterns
	if match, _ := doublestar.Match(pattern, projectPath+"/"); match {
		return true
	}
	return false
}
