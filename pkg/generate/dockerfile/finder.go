package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
)

// Finder allows searching for Dockerfiles in a given directory
type Finder interface {
	Find(dir string) ([]string, error)
}

type finder struct {
	fsWalk func(dir string, fn filepath.WalkFunc) error
}

// NewFinder creates a new Dockerfile Finder
func NewFinder() Finder {
	return &finder{fsWalk: filepath.Walk}
}

// Find returns a list of of found Dockerfile(s) in the given directory
func (f *finder) Find(dir string) ([]string, error) {
	dockerfiles := []string{}
	err := f.fsWalk(dir, func(path string, info os.FileInfo, err error) error {
		if info == nil {
			return nil
		}
		// Skip hidden directories
		if info.IsDir() && strings.HasPrefix(info.Name(), ".") {
			return filepath.SkipDir
		}

		// Add files named Dockerfile
		if info.Name() == "Dockerfile" && err == nil {
			relpath, err := filepath.Rel(dir, path)
			if err == nil {
				dockerfiles = append(dockerfiles, relpath)
			}
		}
		return err
	})
	return dockerfiles, err
}
