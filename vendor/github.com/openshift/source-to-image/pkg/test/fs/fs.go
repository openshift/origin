package fs

import (
	"bytes"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// FakeFileSystem provides a fake filesystem structure for testing
type FakeFileSystem struct {
	ChmodFile  []string
	ChmodMode  os.FileMode
	ChmodError map[string]error

	RenameFrom  string
	RenameTo    string
	RenameError error

	MkdirAllDir   []string
	MkdirAllError error

	MkdirDir   string
	MkdirError error

	ExistsFile   []string
	ExistsResult map[string]bool

	CopySource string
	CopyDest   string
	CopyError  error

	RemoveDirName  string
	RemoveDirError error

	WorkingDirCalled bool
	WorkingDirResult string
	WorkingDirError  error

	OpenFile       string
	OpenFileResult *FakeReadCloser
	OpenContent    string
	OpenError      error
	OpenCloseError error

	CreateFile    string
	CreateContent FakeWriteCloser
	CreateError   error

	WriteFileName    string
	WriteFileError   error
	WriteFileContent string

	ReadlinkName  string
	ReadlinkError error

	SymlinkOldname string
	SymlinkNewname string
	SymlinkError   error

	Files []os.FileInfo

	mutex        sync.Mutex
	keepSymlinks bool
}

// FakeReadCloser provider a fake ReadCloser
type FakeReadCloser struct {
	*bytes.Buffer
	CloseCalled bool
	CloseError  error
}

// Close closes the fake ReadCloser
func (f *FakeReadCloser) Close() error {
	f.CloseCalled = true
	return f.CloseError
}

// FakeWriteCloser provider a fake ReadCloser
type FakeWriteCloser struct {
	bytes.Buffer
}

// Close closes the fake ReadCloser
func (f *FakeWriteCloser) Close() error {
	return nil
}

// ReadDir reads the files in specified directory
func (f *FakeFileSystem) ReadDir(p string) ([]os.FileInfo, error) {
	return f.Files, nil
}

// Lstat provides stats about a single file  (not following symlinks)
func (f *FakeFileSystem) Lstat(p string) (os.FileInfo, error) {
	for _, f := range f.Files {
		if strings.HasSuffix(p, string(filepath.Separator)+f.Name()) {
			return f, nil
		}
	}
	return nil, &os.PathError{Path: p, Err: os.ErrNotExist}
}

// Stat returns a FileInfo describing the named file
func (f *FakeFileSystem) Stat(p string) (os.FileInfo, error) {
	fi, err := f.Lstat(p)
	if err != nil || fi.Mode()&os.ModeSymlink == 0 {
		return fi, err
	}
	p, err = f.Readlink(p)
	if err != nil {
		return nil, err
	}
	return f.Lstat(p)
}

// Chmod manipulates permissions on the fake filesystem
func (f *FakeFileSystem) Chmod(file string, mode os.FileMode) error {
	f.mutex.Lock()
	defer f.mutex.Unlock()
	f.ChmodFile = append(f.ChmodFile, file)
	f.ChmodMode = mode

	return f.ChmodError[file]
}

// Rename renames files on the fake filesystem
func (f *FakeFileSystem) Rename(from, to string) error {
	f.RenameFrom = from
	f.RenameTo = to
	return f.RenameError
}

// MkdirAll creates a new directories on the fake filesystem
func (f *FakeFileSystem) MkdirAll(dirname string) error {
	f.MkdirAllDir = append(f.MkdirAllDir, dirname)
	return f.MkdirAllError
}

// MkdirAllWithPermissions creates a new directories on the fake filesystem
func (f *FakeFileSystem) MkdirAllWithPermissions(dirname string, perm os.FileMode) error {
	f.MkdirAllDir = append(f.MkdirAllDir, dirname)
	return f.MkdirAllError
}

// Mkdir creates a new directory on the fake filesystem
func (f *FakeFileSystem) Mkdir(dirname string) error {
	f.MkdirDir = dirname
	return f.MkdirError
}

// Exists checks if the file exists in fake filesystem
func (f *FakeFileSystem) Exists(file string) bool {
	f.ExistsFile = append(f.ExistsFile, file)
	return f.ExistsResult[file]
}

// Copy copies files on the fake filesystem
func (f *FakeFileSystem) Copy(sourcePath, targetPath string) error {
	f.CopySource = sourcePath
	f.CopyDest = targetPath
	return f.CopyError
}

// CopyContents copies directory contents on the fake filesystem
func (f *FakeFileSystem) CopyContents(sourcePath, targetPath string) error {
	f.CopySource = sourcePath
	f.CopyDest = targetPath
	return f.CopyError
}

// RemoveDirectory removes a directory in the fake filesystem
func (f *FakeFileSystem) RemoveDirectory(dir string) error {
	f.RemoveDirName = dir
	return f.RemoveDirError
}

// CreateWorkingDirectory creates a fake working directory
func (f *FakeFileSystem) CreateWorkingDirectory() (string, error) {
	f.WorkingDirCalled = true
	return f.WorkingDirResult, f.WorkingDirError
}

// Open opens a file
func (f *FakeFileSystem) Open(file string) (io.ReadCloser, error) {
	f.OpenFile = file
	buf := bytes.NewBufferString(f.OpenContent)
	f.OpenFileResult = &FakeReadCloser{
		Buffer:     buf,
		CloseError: f.OpenCloseError,
	}
	return f.OpenFileResult, f.OpenError
}

// Create creates a file
func (f *FakeFileSystem) Create(file string) (io.WriteCloser, error) {
	f.CreateFile = file
	return &f.CreateContent, f.CreateError
}

// WriteFile writes a file
func (f *FakeFileSystem) WriteFile(file string, data []byte) error {
	f.WriteFileName = file
	f.WriteFileContent = string(data)
	return f.WriteFileError
}

// Walk walks the file tree rooted at root, calling walkFn for each file or
// directory in the tree, including root.
func (f *FakeFileSystem) Walk(root string, walkFn filepath.WalkFunc) error {
	return filepath.Walk(root, walkFn)
}

// Readlink reads the destination of a symlink
func (f *FakeFileSystem) Readlink(name string) (string, error) {
	return f.ReadlinkName, f.ReadlinkError
}

// Symlink creates a symlink at newname, pointing to oldname
func (f *FakeFileSystem) Symlink(oldname, newname string) error {
	f.SymlinkOldname, f.SymlinkNewname = oldname, newname
	return f.SymlinkError
}

// KeepSymlinks controls whether to handle symlinks as symlinks or follow
// symlinks and copy files with content
func (f *FakeFileSystem) KeepSymlinks(k bool) {
	f.keepSymlinks = k
}

// ShouldKeepSymlinks informs whether to handle symlinks as symlinks or follow
// symlinks and copy files by content
func (f *FakeFileSystem) ShouldKeepSymlinks() bool {
	return f.keepSymlinks
}
