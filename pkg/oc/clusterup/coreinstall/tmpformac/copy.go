package tmpformac

import (
	"io"
	"os"
	"path/filepath"
)

// CopyFile copies the file at source to dest
// Copied from vendor/github.com/mrunalp/fileutils/fileutils.go
func CopyFile(source string, dest string) error {
	si, err := os.Lstat(source)
	if err != nil {
		return err
	}

	// Handle symlinks
	if si.Mode()&os.ModeSymlink != 0 {
		target, err := os.Readlink(source)
		if err != nil {
			return err
		}
		if err := os.Symlink(target, dest); err != nil {
			return err
		}
	}

	// Handle regular files
	if si.Mode().IsRegular() {
		sf, err := os.Open(source)
		if err != nil {
			return err
		}
		defer sf.Close()

		df, err := os.Create(dest)
		if err != nil {
			return err
		}
		defer df.Close()

		_, err = io.Copy(df, sf)
		if err != nil {
			return err
		}
	}

	// Chown the file
	if err := os.Lchown(dest, os.Getuid(), os.Getgid()); err != nil {
		return err
	}

	// Chmod the file
	if !(si.Mode()&os.ModeSymlink == os.ModeSymlink) {
		if err := os.Chmod(dest, si.Mode()); err != nil {
			return err
		}
	}

	return nil
}

// CopyDirectory copies the files under the source directory
// to dest directory. The dest directory is created if it
// does not exist.
// Copied from vendor/github.com/mrunalp/fileutils/fileutils.go
func CopyDirectory(source string, dest string) error {
	fi, err := os.Stat(source)
	if err != nil {
		return err
	}

	// We have to pick an owner here anyway.
	if err := MkdirAllNewAs(dest, fi.Mode(), os.Getuid(), os.Getgid()); err != nil {
		return err
	}

	return filepath.Walk(source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// Get the relative path
		relPath, err := filepath.Rel(source, path)
		if err != nil {
			return nil
		}

		if info.IsDir() {
			// Skip the source directory.
			if path != source {
				if err := os.Mkdir(filepath.Join(dest, relPath), info.Mode()); err != nil {
					return err
				}

				if err := os.Lchown(filepath.Join(dest, relPath), os.Getuid(), os.Getgid()); err != nil {
					return err
				}
			}
			return nil
		}

		// Copy the file.
		if err := CopyFile(path, filepath.Join(dest, relPath)); err != nil {
			return err
		}

		return nil
	})
}

// MkdirAllNewAs creates a directory (include any along the path) and then modifies
// ownership ONLY of newly created directories to the requested uid/gid. If the
// directories along the path exist, no change of ownership will be performed
// Copied from vendor/github.com/mrunalp/fileutils/idtools.go
func MkdirAllNewAs(path string, mode os.FileMode, ownerUID, ownerGID int) error {
	// make an array containing the original path asked for, plus (for mkAll == true)
	// all path components leading up to the complete path that don't exist before we MkdirAll
	// so that we can chown all of them properly at the end.  If chownExisting is false, we won't
	// chown the full directory path if it exists
	var paths []string
	if _, err := os.Stat(path); err != nil && os.IsNotExist(err) {
		paths = []string{path}
	} else if err == nil {
		// nothing to do; directory path fully exists already
		return nil
	}

	// walk back to "/" looking for directories which do not exist
	// and add them to the paths array for chown after creation
	dirPath := path
	for {
		dirPath = filepath.Dir(dirPath)
		if dirPath == "/" {
			break
		}
		if _, err := os.Stat(dirPath); err != nil && os.IsNotExist(err) {
			paths = append(paths, dirPath)
		}
	}

	if err := os.MkdirAll(path, mode); err != nil && !os.IsExist(err) {
		return err
	}

	// even if it existed, we will chown the requested path + any subpaths that
	// didn't exist when we called MkdirAll
	for _, pathComponent := range paths {
		if err := os.Chown(pathComponent, ownerUID, ownerGID); err != nil {
			return err
		}
	}
	return nil
}

// Copied from vendor/github.com/mrunalp/fileutils/fileutils.go
func major(device uint64) uint64 {
	return (device >> 8) & 0xfff
}

// Copied from vendor/github.com/mrunalp/fileutils/fileutils.go
func minor(device uint64) uint64 {
	return (device & 0xff) | ((device >> 12) & 0xfff00)
}

// Copied from vendor/github.com/mrunalp/fileutils/fileutils.go
func mkdev(major int64, minor int64) uint32 {
	return uint32(((minor & 0xfff00) << 12) | ((major & 0xfff) << 8) | (minor & 0xff))
}
