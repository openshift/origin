//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package remoteexec

import (
	"errors"
)

// Result is used to capture the result of running a command
// on a remote system.
type Result struct {
	Completed  bool
	Output     string
	ErrOutput  string
	Err        error
	ExitStatus int
}

// Ok returns a boolean indicating that the command ran and
// completed successfully.
func (r Result) Ok() bool {
	return r.Completed && r.Err == nil && r.ExitStatus == 0
}

// Error allows an error result to be treated like an error type.
// Do not assume all results are true errors.
func (r Result) Error() string {
	// to be more compatible with older versions of heketi we use
	// the error output (stderr) of commands unless that
	// string is empty
	if r.ErrOutput != "" {
		return r.ErrOutput
	}
	if r.Err != nil {
		return r.Err.Error()
	}
	return ""
}

// Results is used for a grouping of Result items.
type Results []Result

// SquashErrors converts a fully fledged results array into a
// simple array of strings and a single error code. This
// is the style most code in heketi currently expects and
// exists to ease refactoring.
func (rs Results) SquashErrors() ([]string, error) {
	outputs := make([]string, len(rs))
	for i, result := range rs {
		if !result.Completed {
			break
		}
		if result.Err != nil || result.ExitStatus != 0 {
			return nil, errors.New(result.ErrOutput)
		}
		outputs[i] = result.Output
	}
	return outputs, nil
}

// Ok returns a boolean indicating that all completed commands
// were successful.
func (rs Results) Ok() bool {
	for _, r := range rs {
		if !r.Completed {
			continue
		}
		if !r.Ok() {
			return false
		}
	}
	return true
}

// FirstErrorIndexed returns the first error found in the Results
// along with its index. If no error is found -1 and nil are
// returned.
func (rs Results) FirstErrorIndexed() (int, error) {
	for i, r := range rs {
		if !r.Completed {
			continue
		}
		if !r.Ok() {
			return i, r
		}
	}
	return -1, nil
}

// FirstError returns the first error found in the Results.
// If no error is found nil is returned.
func (rs Results) FirstError() error {
	_, err := rs.FirstErrorIndexed()
	return err
}

// AnyError takes both the Results and any communication error
// condition from an ExecCommands style function and returns
// the first error condition found, or nil if none found.
func AnyError(res Results, err error) error {
	if err != nil {
		return err
	}
	return res.FirstError()
}
