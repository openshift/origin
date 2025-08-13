// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package debug

import (
	"io"
	"os"
	"path"
	"sync"
)

// FileProvider implements a debugging provider that creates a real file for
// every call to NewFile. It maintains a list of all files that it creates,
// such that it can close them when its Flush function is called.
type FileProvider struct {
	Path string

	mu    sync.Mutex
	files []*os.File
}

func (fp *FileProvider) NewFile(p string) io.WriteCloser {
	f, err := os.Create(path.Join(fp.Path, p))
	if err != nil {
		panic(err)
	}

	fp.mu.Lock()
	defer fp.mu.Unlock()
	fp.files = append(fp.files, f)

	return NewFileWriterCloser(f, p)
}

func (fp *FileProvider) Flush() {
	fp.mu.Lock()
	defer fp.mu.Unlock()
	for _, f := range fp.files {
		f.Close()
	}
}

type FileWriterCloser struct {
	f *os.File
	p string
}

func NewFileWriterCloser(f *os.File, p string) *FileWriterCloser {
	return &FileWriterCloser{
		f,
		p,
	}
}

func (fwc *FileWriterCloser) Write(p []byte) (n int, err error) {
	return fwc.f.Write(Scrub(p))
}

func (fwc *FileWriterCloser) Close() error {
	return fwc.f.Close()
}
