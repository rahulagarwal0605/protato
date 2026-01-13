package utils

import (
	"fmt"
	"os"
)

// DirNotExists checks if a directory does not exist.
func DirNotExists(dirPath string) bool {
	_, err := os.Stat(dirPath)
	return os.IsNotExist(err)
}

// FileExists checks if a file exists at the given path.
func FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return err == nil
}

// CreateDir creates a directory if it doesn't exist.
func CreateDir(path, name string) error {
	if err := os.MkdirAll(path, 0755); err != nil {
		return fmt.Errorf("create %s dir: %w", name, err)
	}
	return nil
}
