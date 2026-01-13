package utils

import (
	"errors"
	"fmt"
	"io/fs"
	"strings"

	"github.com/rahulagarwal0605/protato/internal/constants"
)

// ValidateProjectPath validates a project path.
// A valid project path:
// - Cannot be empty
// - Cannot contain backslashes
// - Cannot have leading or trailing slashes
// - Must be a valid filesystem path
func ValidateProjectPath(p string) error {
	if p == "" {
		return errors.New(constants.ErrMsgProjectPathEmpty)
	}
	if strings.Contains(p, "\\") {
		return errors.New(constants.ErrMsgProjectPathBackslash)
	}
	if strings.HasPrefix(p, "/") || strings.HasSuffix(p, "/") {
		return errors.New(constants.ErrMsgProjectPathSlash)
	}
	if !fs.ValidPath(p) {
		return errors.New(constants.ErrMsgProjectPathInvalid)
	}
	return nil
}

// ProjectsOverlap checks if any two project paths overlap.
// Two paths overlap if one is a prefix of the other (e.g., "a/b" and "a/b/c" overlap).
func ProjectsOverlap(projects []string) error {
	for i, p1 := range projects {
		for j, p2 := range projects {
			if i == j {
				continue
			}
			if strings.HasPrefix(p1+"/", p2+"/") || strings.HasPrefix(p2+"/", p1+"/") {
				return fmt.Errorf("projects overlap: %s and %s", p1, p2)
			}
		}
	}
	return nil
}

// PathBelongsToAny checks if a path belongs to any of the given base paths.
// A path belongs to a base path if it starts with the base path followed by "/" or equals the base path.
func PathBelongsToAny(path string, basePaths map[string]bool) bool {
	for base := range basePaths {
		if strings.HasPrefix(path, base+"/") || path == base {
			return true
		}
	}
	return false
}
