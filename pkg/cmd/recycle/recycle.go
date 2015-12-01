package recycle

import "os"

// Recycle recursively deletes files and folders within the given path. It does not delete the path itself.
func Recycle(dir string) error {
	return newWalker(func(path string, info os.FileInfo) error {
		// Leave the root dir alone
		if path == dir {
			return nil
		}

		// Delete all subfiles/subdirs
		return os.Remove(path)
	}).Walk(dir)
}
