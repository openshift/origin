package fsnotification

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"

	"github.com/fsnotify/fsnotify"
)

// AddRecursiveWatch handles adding watches recursively for the path provided
// and its subdirectories.  If a non-directory is specified, this call is a no-op.
// Recursive logic from https://github.com/bronze1man/kmg/blob/master/fsnotify/Watcher.go
func AddRecursiveWatch(watcher *fsnotify.Watcher, path string) error {
	file, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("error introspecting path %s: %v", path, err)
	}
	if !file.IsDir() {
		return nil
	}

	folders, err := getSubFolders(path)
	for _, v := range folders {
		glog.V(5).Infof("adding watch on path %s", v)
		err = watcher.Add(v)
		if err != nil {
			// "no space left on device" issues are usually resolved via
			// $ sudo sysctl fs.inotify.max_user_watches=65536
			return fmt.Errorf("error adding watcher for path %s: %v", v, err)
		}
	}
	return nil
}

// getSubFolders recursively retrieves all subfolders of the specified path.
func getSubFolders(path string) (paths []string, err error) {
	err = filepath.Walk(path, func(newPath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if info.IsDir() {
			paths = append(paths, newPath)
		}
		return nil
	})
	return paths, err
}
