// © Broadcom. All Rights Reserved.
// The term “Broadcom” refers to Broadcom Inc. and/or its subsidiaries.
// SPDX-License-Identifier: Apache-2.0

package debug

import (
	"io"
	"regexp"
)

// Provider specified the interface types must implement to be used as a
// debugging sink. Having multiple such sink implementations allows it to be
// changed externally (for example when running tests).
type Provider interface {
	NewFile(s string) io.WriteCloser
	Flush()
}

// ReadCloser is a struct that satisfies the io.ReadCloser interface
type ReadCloser struct {
	io.Reader
	io.Closer
}

// NewTeeReader wraps io.TeeReader and patches through the Close() function.
func NewTeeReader(rc io.ReadCloser, w io.Writer) io.ReadCloser {
	return ReadCloser{
		Reader: io.TeeReader(rc, w),
		Closer: rc,
	}
}

var currentProvider Provider = nil
var scrubPassword = regexp.MustCompile(`<password>(.*)</password>`)

func SetProvider(p Provider) {
	if currentProvider != nil {
		currentProvider.Flush()
	}
	currentProvider = p
}

// Enabled returns whether debugging is enabled or not.
func Enabled() bool {
	return currentProvider != nil
}

// NewFile dispatches to the current provider's NewFile function.
func NewFile(s string) io.WriteCloser {
	return currentProvider.NewFile(s)
}

// Flush dispatches to the current provider's Flush function.
func Flush() {
	currentProvider.Flush()
}

func Scrub(in []byte) []byte {
	return scrubPassword.ReplaceAll(in, []byte(`<password>********</password>`))
}
