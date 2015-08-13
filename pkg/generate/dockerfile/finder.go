package dockerfile

import (
	"os"
	"path/filepath"
	"strings"
)

type Tester interface {
	Has(dir string) (string, bool, error)
}

type StatFunc func(path string) (os.FileInfo, error)

func (t StatFunc) Has(dir string) (string, bool, error) {
	path := filepath.Join(dir, "Dockerfile")
	_, err := t(path)
	if os.IsNotExist(err) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return path, true, nil
}

func NewTester() Tester {
	return StatFunc(os.Stat)
}

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
			relpath, relerr := filepath.Rel(dir, path)
			if relerr == nil {
				dockerfiles = append(dockerfiles, relpath)
			} else {
				return relerr
			}
		}
		return err
	})
	return dockerfiles, err
}
