package test

import (
	"io"
)

type FakeTar struct {
	CreateTarBase   string
	CreateTarDir    string
	CreateTarResult string
	CreateTarError  error

	ExtractTarDir    string
	ExtractTarReader io.Reader
	ExtractTarError  error
}

func (f *FakeTar) CreateTarFile(base, dir string) (string, error) {
	f.CreateTarBase = base
	f.CreateTarDir = dir
	return f.CreateTarResult, f.CreateTarError
}

func (f *FakeTar) ExtractTarStream(dir string, reader io.Reader) error {
	f.ExtractTarDir = dir
	f.ExtractTarReader = reader
	return f.ExtractTarError
}
