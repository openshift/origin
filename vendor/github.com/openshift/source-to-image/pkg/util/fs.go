package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"

	utilglog "github.com/openshift/source-to-image/pkg/util/glog"

	"github.com/openshift/source-to-image/pkg/errors"
)

var glog = utilglog.StderrLog

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
	if runtime.GOOS == "windows" {
		return nil
	}
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

// Copy copies the source to a destination.
// If the source is a file, then the destination has to be a file as well,
// otherwise you will get an error.
// If the source is a directory, then the destination has to be a directory and
// we copy the content of the source directory to destination directory
// recursively.
func (h *fs) Copy(source string, dest string) (err error) {
	sourcefile, err := os.Open(source)
	if err != nil {
		return err
	}
	defer sourcefile.Close()
	sourceinfo, err := os.Stat(source)
	if err != nil {
		return err
	}

	if sourceinfo.IsDir() {
		glog.V(5).Infof("D %q -> %q", source, dest)
		return h.CopyContents(source, dest)
	}

	destinfo, _ := os.Stat(dest)
	if destinfo != nil && destinfo.IsDir() {
		return fmt.Errorf("destination must be full path to a file, not directory")
	}
	destfile, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer destfile.Close()
	glog.V(5).Infof("F %q -> %q", source, dest)
	if _, err := io.Copy(destfile, sourcefile); err != nil {
		return err
	}

	return h.Chmod(dest, sourceinfo.Mode())
}

// CopyContents copies the content of the source directory to a destination
// directory.
// If the destination directory does not exists, it will be created.
// The source directory itself will not be copied, only its content. If you
// want this behavior, the destination must include the source directory name.
func (h *fs) CopyContents(src string, dest string) (err error) {
	sourceinfo, err := os.Stat(src)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(dest, sourceinfo.Mode()); err != nil {
		return err
	}
	directory, err := os.Open(src)
	if err != nil {
		return err
	}
	objects, err := directory.Readdir(-1)
	if err != nil {
		return err
	}
	for _, obj := range objects {
		source := path.Join(src, obj.Name())
		destination := path.Join(dest, obj.Name())
		if err := h.Copy(source, destination); err != nil {
			return err
		}

	}
	return
}

// RemoveDirectory removes the specified directory and all its contents
func (h *fs) RemoveDirectory(dir string) error {
	glog.V(2).Infof("Removing directory '%s'", dir)

	// HACK: If deleting a directory in windows, call out to the system to do the deletion
	// TODO: Remove this workaround when we switch to go 1.7 -- os.RemoveAll should
	// be fixed for Windows in that release.
	if runtime.GOOS == "windows" {
		cmd := exec.Command("cmd.exe", "/c", fmt.Sprintf("rd /s /q %s", dir))
		output, err := cmd.Output()
		if err != nil {
			glog.Errorf("Error removing directory %q: %v %s", dir, err, string(output))
			return err
		}
		return nil
	}

	err := os.RemoveAll(dir)
	if err != nil {
		glog.Errorf("Error removing directory '%s': %v", dir, err)
	}
	return err
}

// CreateWorkingDirectory creates a directory to be used for STI
func (h *fs) CreateWorkingDirectory() (directory string, err error) {
	directory, err = ioutil.TempDir("", "s2i")
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
