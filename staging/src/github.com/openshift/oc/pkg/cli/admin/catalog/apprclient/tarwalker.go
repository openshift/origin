package apprclient

import (
	"archive/tar"
	"bytes"
	"errors"
	"fmt"
	"io"
)

// Processor is an interface that wraps the Process method.
//
// Process is called by tar walker to notify of an element just discovered.
//
// if done is set to true then tar walker will exit from the walk loop.
// Otherwise, it will continue
type Processor interface {
	Process(header *tar.Header, manifestName, workingDirectory string, reader io.Reader) (done bool, err error)
}

type tarWalker struct {
}

func (*tarWalker) Walk(raw []byte, manifestName, workingDirectory string, processor Processor) error {
	if raw == nil || processor == nil {
		return errors.New("invalid argument specified to Walk")
	}

	reader := tar.NewReader(bytes.NewBuffer(raw))

	for true {
		header, err := reader.Next()

		if err != nil {
			if err == io.EOF {
				return nil
			}

			return errors.New(fmt.Sprintf("extraction of tar ball failed - %s", err.Error()))
		}

		if header == nil {
			// We shouldn't be here, being extra defensive here!
			continue
		}

		switch header.Typeflag {
		case tar.TypeReg:
			// It's a regular file
			done, err := processor.Process(header, manifestName, workingDirectory, reader)
			if err != nil {
				return fmt.Errorf("error happened while processing tar file - %s", err.Error())
			}

			if done {
				return nil
			}

		case tar.TypeDir:
			done, err := processor.Process(header, manifestName, workingDirectory, reader)
			if err != nil {
				return fmt.Errorf("error happened while processing directory - %s", err.Error())
			}

			if done {
				return nil
			}
		}
	}

	return nil
}
