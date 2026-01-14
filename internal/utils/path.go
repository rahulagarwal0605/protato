package utils

import (
	"path"
	"path/filepath"
	"strings"
)

// RelPathToSlash calculates a relative path and converts it to forward slashes.
func RelPathToSlash(base, target string) (string, error) {
	rel, err := filepath.Rel(base, target)
	if err != nil {
		return "", err
	}
	return filepath.ToSlash(rel), nil
}

// TrimPathPrefix removes a prefix from a path.
func TrimPathPrefix(path, prefix string) string {
	return strings.TrimPrefix(path, prefix+"/")
}

// AbsPath returns the absolute path, returning error if conversion fails.
func AbsPath(path string) (string, error) {
	return filepath.Abs(path)
}

// ExtractParentPath extracts the parent path by removing the last component.
// Returns empty string if path has less than 2 components.
// Example: "a/b/c" -> "a/b", "a/b" -> "a", "a" -> ""
func ExtractParentPath(path string) string {
	parts := strings.Split(path, "/")
	if len(parts) < 2 {
		return ""
	}
	return strings.Join(parts[:len(parts)-1], "/")
}

// RemovePathPrefixIfExists removes a prefix from a path if it exists.
// Returns the path without prefix if prefix matches, empty string if prefix doesn't match.
// Example: RemovePathPrefixIfExists("proto/common/file.proto", "proto") -> "common/file.proto"
//          RemovePathPrefixIfExists("other/file.proto", "proto") -> ""
func RemovePathPrefixIfExists(path, prefix string) string {
	if prefix == "" {
		return path
	}
	prefixWithSlash := prefix + "/"
	if !strings.HasPrefix(path, prefixWithSlash) {
		return ""
	}
	return strings.TrimPrefix(path, prefixWithSlash)
}

// JoinPathPrefix joins a prefix with additional path parts.
// Example: JoinPathPrefix("proto", "common", "file.proto") -> "proto/common/file.proto"
func JoinPathPrefix(prefix string, parts ...string) string {
	return path.Join(append([]string{prefix}, parts...)...)
}
