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
