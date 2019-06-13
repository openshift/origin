package recycle

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"sort"
)

type walkFunc func(path string, info os.FileInfo) error

// Walker visits a directory tree, depth-first, calling walkFn for every file/directory.
// It calls setfsuid with the owning UID of each directory before calling walkFn for the direct children of that directory.
type walker struct {
	walkFn walkFunc

	// fsuid holds our current fsuid
	fsuid int64

	// lstat is for testing, defaults to os.Lstat
	lstat func(path string) (os.FileInfo, error)

	// getuid is for testing, defaults to fileinfo.Sys().(*syscall.Stat_t).Uid
	getuid func(info os.FileInfo) (int64, error)

	// setfsuid is for testing, defaults to syscall.Setfsuid
	setfsuid func(uid int) error

	// readDirNames is for testing, defaults to readDirNames
	readDirNames func(dirname string) ([]string, error)
}

type walkError struct {
	path      string
	info      os.FileInfo
	operation string
	err       error
}

func (w walkError) Error() string {
	var mode interface{} = "unknown"
	if w.info != nil {
		mode = w.info.Mode()
	}
	return fmt.Sprintf("%s (%s), %s: %s", w.path, mode, w.operation, w.err)
}

func makeWalkError(path string, info os.FileInfo, err error, operation string) error {
	if _, isWalkError := err.(walkError); isWalkError {
		return err
	}
	return walkError{path, info, operation, err}
}

func newWalker(walkFn walkFunc) *walker {
	return &walker{
		walkFn:       walkFn,
		fsuid:        int64(os.Getuid()), // default to the uid of the process
		lstat:        os.Lstat,
		getuid:       getuid,
		setfsuid:     setfsuid,
		readDirNames: readDirNames,
	}
}

func (w *walker) Walk(root string) error {
	// Lock threads, so our Setfsuid calls always apply to the same thread
	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	// The launching process must have the ability to stat the root dir
	info, err := w.lstat(root)
	if err != nil {
		return makeWalkError(root, info, err, "lstat root dir")
	}

	// become the root dir's owner to begin
	err = w.becomeOwner(info)
	if err != nil {
		return makeWalkError(root, info, err, "becoming root dir owner")
	}

	return w.walk(root, info)
}

func (w *walker) walk(path string, info os.FileInfo) error {
	var err error

	// Descend first
	if info.IsDir() {
		// Remember our current fsuid
		previousFSuid := w.fsuid

		// become the dir's owner, in order to list/rmdir/unlink child files
		err = w.becomeOwner(info)
		if err != nil {
			return makeWalkError(path, info, err, "becoming dir owner")
		}

		// read dir info
		names, err := w.readDirNames(path)
		if err != nil {
			return makeWalkError(path, info, err, fmt.Sprintf("reading dir names as %d", w.fsuid))
		}

		// visit files
		for _, name := range names {
			filename := filepath.Join(path, name)

			fileInfo, err := w.lstat(filename)
			if err != nil {
				return makeWalkError(path, info, err, fmt.Sprintf("lstat child as %d", w.fsuid))
			}

			err = w.walk(filename, fileInfo)
			if err != nil {
				return err
			}
		}

		// Return to our previous fsuid, in order to rmdir the current directory
		err = w.becomeUid(previousFSuid)
		if err != nil {
			return makeWalkError(path, info, err, "returning to previous uid")
		}
	}

	// visit the current file
	err = w.walkFn(path, info)
	if err != nil {
		return makeWalkError(path, info, err, "calling walkFn")
	}

	return nil
}

func (w *walker) becomeOwner(info os.FileInfo) error {
	// get the UID
	uid, err := w.getuid(info)
	if err != nil {
		return err
	}

	return w.becomeUid(uid)
}

func (w *walker) becomeUid(uid int64) error {
	// if we already were the UID, no-op
	if w.fsuid == uid {
		return nil
	}

	// become the UID
	if err := w.setfsuid(int(uid)); err != nil {
		return err
	}

	// remember the last UID we became
	w.fsuid = uid

	return nil
}

// readDirNames reads the directory named by dirname and returns a sorted list of directory entries.
func readDirNames(dirname string) ([]string, error) {
	f, err := os.Open(dirname)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	names, err := f.Readdirnames(-1)
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	return names, nil
}
