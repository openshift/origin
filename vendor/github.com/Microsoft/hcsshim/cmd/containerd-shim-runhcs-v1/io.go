package main

import (
	"io"
)

// upstreamIO is an interface describing the IO to connect to above the shim.
// Depending on the callers settings there may be no opened IO.
type upstreamIO interface {
	// Close closes all open io.
	//
	// This call is idempotent and safe to call multiple times.
	Close()
	// CloseStdin closes just `Stdin()` if open.
	//
	// This call is idempotent and safe to call multiple times.
	CloseStdin()
	// Stdin returns the open `stdin` reader. If `stdin` was never opened this
	// will return `nil`.
	Stdin() io.Reader
	// StdinPath returns the original path used to open the `Stdin()` reader.
	StdinPath() string
	// Stdout returns the open `stdout` writer. If `stdout` was never opened
	// this will return `nil`.
	Stdout() io.Writer
	// StdoutPath returns the original path used to open the `Stdout()` writer.
	StdoutPath() string
	// Stderr returns the open `stderr` writer. If `stderr` was never opened
	// this will return `nil`.
	Stderr() io.Writer
	// StderrPath returns the original path used to open the `Stderr()` writer.
	StderrPath() string
	// Terminal returns `true` if the connection is emulating a terminal. If
	// `true` `Stderr()` will always return `nil` and `StderrPath()` will always
	// return `""`.
	Terminal() bool
}
