package app

import (
	"os"
)

// isFile returns true if the passed-in argument is a file in the filesystem
func isFile(name string) (bool, error) {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir(), err
}

// IsDirectory returns true if the passed-in argument is a directory in the filesystem
func IsDirectory(name string) (bool, error) {
	info, err := os.Stat(name)
	return err == nil && info.IsDir(), err
}
