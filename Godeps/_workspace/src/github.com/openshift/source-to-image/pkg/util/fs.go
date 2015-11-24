package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"github.com/golang/glog"

	"github.com/openshift/source-to-image/pkg/errors"
)

// FileSystem allows STI to work with the file system and
// perform tasks such as creating and deleting directories
type FileSystem interface {
	Chmod(file string, mode os.FileMode) error
	Rename(from, to string) error
	MkdirAll(dirname string) error
	Mkdir(dirname string) error
	Exists(file string) bool
	Copy(sourcePath, targetPath string) error
	CopyContents(sourcePath, targetPath string) error
	RemoveDirectory(dir string) error
	CreateWorkingDirectory() (string, error)
	Open(file string) (io.ReadCloser, error)
	WriteFile(file string, data []byte) error
	ReadDir(string) ([]os.FileInfo, error)
	Stat(string) (os.FileInfo, error)
}

// NewFileSystem creates a new instance of the default FileSystem
// implementation
func NewFileSystem() FileSystem {
	return &fs{
		runner: NewCommandRunner(),
	}
}

type fs struct {
	runner CommandRunner
}

func (h *fs) Stat(path string) (os.FileInfo, error) {
	return os.Stat(path)
}

func (h *fs) ReadDir(path string) ([]os.FileInfo, error) {
	return ioutil.ReadDir(path)
}

// Chmod sets the file mode
func (h *fs) Chmod(file string, mode os.FileMode) error {
	return os.Chmod(file, mode)
}

// Rename renames or moves a file
func (h *fs) Rename(from, to string) error {
	return os.Rename(from, to)
}

// MkdirAll creates the directory and all its parents
func (h *fs) MkdirAll(dirname string) error {
	return os.MkdirAll(dirname, 0700)
}

// Mkdir creates the specified directory
func (h *fs) Mkdir(dirname string) error {
	return os.Mkdir(dirname, 0700)
}

// Exists determines whether the given file exists
func (h *fs) Exists(file string) bool {
	_, err := os.Stat(file)
	return err == nil
}

// Copy copies a set of files from sourcePath to targetPath
func (h *fs) Copy(sourcePath string, targetPath string) error {
	if _, err := os.Stat(sourcePath); err != nil {
		return err
	}

	info, err := os.Stat(targetPath)

	if err != nil || (info != nil && !info.IsDir()) {
		err = os.Mkdir(targetPath, 0700)
		if err != nil {
			return err
		}

		targetPath = filepath.Join(targetPath, filepath.Base(sourcePath))
	}
	// TODO: Use the appropriate command for Windows
	glog.V(5).Infof("cp -a %s %s", sourcePath, targetPath)
	return h.runner.Run("cp", "-a", sourcePath, targetPath)
}

func (h *fs) CopyContents(sourcePath string, targetPath string) error {
	info, err := os.Stat(sourcePath)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return fmt.Errorf("source path %s is not a directory", sourcePath)
	}
	if !strings.HasSuffix(sourcePath, string(filepath.Separator)) {
		sourcePath += string(filepath.Separator)
	}
	return h.Copy(sourcePath, targetPath)
}

// RemoveDirectory removes the specified directory and all its contents
func (h *fs) RemoveDirectory(dir string) error {
	glog.V(2).Infof("Removing directory '%s'", dir)

	err := os.RemoveAll(dir)
	if err != nil {
		glog.Errorf("Error removing directory '%s': %v", dir, err)
	}
	return err
}

// CreateWorkingDirectory creates a directory to be used for STI
func (h *fs) CreateWorkingDirectory() (directory string, err error) {
	directory, err = ioutil.TempDir("", "sti")
	if err != nil {
		return "", errors.NewWorkDirError(directory, err)
	}

	return directory, err
}

// Open opens a file and returns a ReadCloser interface to that file
func (h *fs) Open(filename string) (io.ReadCloser, error) {
	return os.Open(filename)
}

// Write opens a file and writes data to it, returning error if such occurred
func (h *fs) WriteFile(filename string, data []byte) error {
	return ioutil.WriteFile(filename, data, 0700)
}
