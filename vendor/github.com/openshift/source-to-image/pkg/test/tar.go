package test

import (
	"errors"
	"io"
	"path/filepath"
	"regexp"
	"sync"

	"github.com/openshift/source-to-image/pkg/tar"
)

// FakeTar provides a fake UNIX tar interface
type FakeTar struct {
	CreateTarBase   string
	CreateTarDir    string
	CreateTarResult string
	CreateTarError  error

	ExtractTarDir    string
	ExtractTarReader io.Reader
	ExtractTarError  error

	lock sync.Mutex
}

// Copy returns a copy of the FakeTar object
func (f *FakeTar) Copy() *FakeTar {
	f.lock.Lock()
	defer f.lock.Unlock()
	// copy everything except .lock...
	n := &FakeTar{
		CreateTarBase:    f.CreateTarBase,
		CreateTarDir:     f.CreateTarDir,
		CreateTarResult:  f.CreateTarResult,
		CreateTarError:   f.CreateTarError,
		ExtractTarDir:    f.ExtractTarDir,
		ExtractTarReader: f.ExtractTarReader,
		ExtractTarError:  f.ExtractTarError,
	}
	return n
}

// CreateTarFile creates a new fake UNIX tar file
func (f *FakeTar) CreateTarFile(base, dir string) (string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.CreateTarBase = base
	f.CreateTarDir = dir
	return f.CreateTarResult, f.CreateTarError
}

// ExtractTarStreamWithLogging streams a content of fake tar
func (f *FakeTar) ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.ExtractTarDir = dir
	f.ExtractTarReader = reader
	return f.ExtractTarError
}

// ExtractTarStreamFromTarReader streams a content of fake tar from a tar.Reader
func (f *FakeTar) ExtractTarStreamFromTarReader(dir string, tarReader tar.Reader, logger io.Writer) error {
	return errors.New("not implemented")
}

// ExtractTarStream streams a content of fake tar
func (f *FakeTar) ExtractTarStream(dir string, reader io.Reader) error {
	return f.ExtractTarStreamWithLogging(dir, reader, nil)
}

// SetExclusionPattern sets the exclusion pattern
func (f *FakeTar) SetExclusionPattern(*regexp.Regexp) {
}

// StreamFileAsTar streams a single file as a TAR archive into specified writer.
func (f *FakeTar) StreamFileAsTar(string, string, io.Writer) error {
	return nil
}

// StreamFileAsTarWithCallback streams a single file as a TAR archive into specified writer.
func (f *FakeTar) StreamFileAsTarWithCallback(source, name string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error {
	return errors.New("not implemented")
}

// StreamDirAsTar streams a directory as a TAR archive into specified writer.
func (f *FakeTar) StreamDirAsTar(string, string, io.Writer) error {
	return nil
}

// StreamDirAsTarWithCallback streams a directory as a TAR archive into specified writer.
func (f *FakeTar) StreamDirAsTarWithCallback(source string, writer io.Writer, walkFn filepath.WalkFunc, modifyInplace bool) error {
	return errors.New("not implemented")
}

// CreateTarStreamWithLogging creates a tar from the given directory and streams
// it to the given writer.
func (f *FakeTar) CreateTarStreamWithLogging(dir string, includeDirInPath bool, writer io.Writer, logger io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.CreateTarDir = dir
	return f.CreateTarError
}

// CreateTarStream creates a tar from the given directory and streams it to the
// given writer.
func (f *FakeTar) CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error {
	return f.CreateTarStreamWithLogging(dir, includeDirInPath, writer, nil)
}
