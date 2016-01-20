package test

import (
	"io"
	"regexp"
	"sync"
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

func (f *FakeTar) Copy() *FakeTar {
	f.lock.Lock()
	defer f.lock.Unlock()
	n := *f
	return &n
}

// CreateTarFile creates a new fake UNIX tar file
func (f *FakeTar) CreateTarFile(base, dir string) (string, error) {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.CreateTarBase = base
	f.CreateTarDir = dir
	return f.CreateTarResult, f.CreateTarError
}

// ExtractTarStream streams a content of fake tar
func (f *FakeTar) ExtractTarStreamWithLogging(dir string, reader io.Reader, logger io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.ExtractTarDir = dir
	f.ExtractTarReader = reader
	return f.ExtractTarError
}

func (f *FakeTar) ExtractTarStream(dir string, reader io.Reader) error {
	return f.ExtractTarStreamWithLogging(dir, reader, nil)
}

func (f *FakeTar) SetExclusionPattern(*regexp.Regexp) {
}

func (f *FakeTar) StreamFileAsTar(string, string, io.Writer) error {
	return nil
}

func (f *FakeTar) StreamDirAsTar(string, string, io.Writer) error {
	return nil
}

func (f *FakeTar) CreateTarStreamWithLogging(dir string, includeDirInPath bool, writer io.Writer, logger io.Writer) error {
	f.lock.Lock()
	defer f.lock.Unlock()
	f.CreateTarDir = dir
	return f.CreateTarError
}

func (f *FakeTar) CreateTarStream(dir string, includeDirInPath bool, writer io.Writer) error {
	return f.CreateTarStreamWithLogging(dir, includeDirInPath, writer, nil)
}
