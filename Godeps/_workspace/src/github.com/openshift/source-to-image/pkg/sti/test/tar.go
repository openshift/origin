package test

import (
	"io"
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
}

// CreateTarFile creates a new fake UNIX tar file
func (f *FakeTar) CreateTarFile(base, dir string) (string, error) {
	f.CreateTarBase = base
	f.CreateTarDir = dir
	return f.CreateTarResult, f.CreateTarError
}

// ExtractTarStream streams a content of fake tar
func (f *FakeTar) ExtractTarStream(dir string, reader io.Reader) error {
	f.ExtractTarDir = dir
	f.ExtractTarReader = reader
	return f.ExtractTarError
}
