package apprclient

import (
	"archive/tar"
	"io"
	"os"
	"path/filepath"
)

const (
	directoryPerm = 0755
	fileFlag      = os.O_CREATE | os.O_RDWR
)

// NewBundleProcessor is a bundleProcessor constructor
func NewBundleProcessor() *bundleProcessor {
	return &bundleProcessor{}
}

type bundleProcessor struct {
}

// Process takes an item of the tar ball and writes it to the underlying file
// system.
func (w *bundleProcessor) Process(header *tar.Header, manifestName, workingDirectory string, reader io.Reader) (done bool, err error) {

	namedManifestDirectory := filepath.Join(workingDirectory, manifestName)
	target := filepath.Join(namedManifestDirectory, header.Name)

	if header.Typeflag == tar.TypeDir {
		if _, err = os.Stat(target); err == nil {
			return
		}

		err = os.MkdirAll(target, directoryPerm)
		return
	}

	if header.Typeflag != tar.TypeReg {
		return
	}

	// It's a file.
	f, err := os.OpenFile(target, fileFlag, os.FileMode(header.Mode))
	if err != nil {
		return
	}

	defer f.Close()

	_, err = io.Copy(f, reader)
	return
}
