package app

import (
	"os"
)

// isFile returns true if the passed-in argument is a file in the filesystem
func isFile(name string) bool {
	info, err := os.Stat(name)
	return err == nil && !info.IsDir()
}

// isDirectory returns true if the passed-in argument is a directory in the filesystem
func isDirectory(name string) bool {
	info, err := os.Stat(name)
	return err == nil && info.IsDir()
}
