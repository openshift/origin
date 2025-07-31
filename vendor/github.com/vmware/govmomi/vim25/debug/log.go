// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package debug

import (
	"fmt"
	"io"
	"os"
)

type LogWriterCloser struct {
}

func NewLogWriterCloser() *LogWriterCloser {
	return &LogWriterCloser{}
}

func (lwc *LogWriterCloser) Write(p []byte) (n int, err error) {
	fmt.Fprint(os.Stderr, string(Scrub(p)))
	return len(p), nil
}

func (lwc *LogWriterCloser) Close() error {
	return nil
}

type LogProvider struct {
}

func (s *LogProvider) NewFile(p string) io.WriteCloser {
	return NewLogWriterCloser()
}

func (s *LogProvider) Flush() {
}
