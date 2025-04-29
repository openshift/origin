package check

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"path/filepath"
)

// AllFilesystemChecks returns a list of filesystem checks to be performed on the image filesystem.
func AllFilesystemChecks() []FilesystemCheck {
	return []FilesystemCheck{
		FilesystemHasDirectoriesWith("configs", "tmp/cache"),
		FilesystemHasPogrebCacheWith("tmp/cache"),
	}
}

// FilesystemHasDirectoriesWith checks if the specified paths exist and are directories in the image filesystem.
func FilesystemHasDirectoriesWith(paths ...string) FilesystemCheck {
	return FilesystemCheck{
		Name: "FilesystemHasDirectories" + fmt.Sprintf(":(%q) ", paths),
		Fn: func(ctx context.Context, imageFS fs.FS) error {
			var errs []error
			for _, path := range paths {
				stat, err := fs.Stat(imageFS, path)
				if err != nil {
					errs = append(errs, err)
					continue
				}
				if !stat.IsDir() {
					errs = append(errs, fmt.Errorf("%q is not a directory", path))
				}
			}
			return errors.Join(errs...)
		},
	}
}

// FilesystemHasPogrebCacheWith checks if the pogreb cache directory exists and is a directory.
func FilesystemHasPogrebCacheWith(pathDir string) FilesystemCheck {
	return FilesystemCheck{
		Name: "FilesystemHasPogrebCache",
		Fn: func(ctx context.Context, imageFS fs.FS) error {
			cacheDir := filepath.Join(pathDir, "pogreb.v1")
			stat, err := fs.Stat(imageFS, cacheDir)
			if err != nil {
				return err
			}
			if !stat.IsDir() {
				return fmt.Errorf("%q is not a directory", cacheDir)
			}
			return nil
		},
	}
}
