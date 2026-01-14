package utils

import (
	"strings"
)

// NormalizeGitURL normalizes a Git URL to HTTPS format.
// Converts SSH URLs (git@host:path) to HTTPS format and removes .git suffix.
func NormalizeGitURL(url string) string {
	// Convert SSH URLs to HTTPS
	if strings.HasPrefix(url, "git@") {
		// git@github.com:org/repo.git -> https://github.com/org/repo.git
		url = strings.Replace(url, ":", "/", 1)
		url = strings.Replace(url, "git@", "https://", 1)
	}
	// Remove .git suffix if present
	url = strings.TrimSuffix(url, ".git")
	return url
}
