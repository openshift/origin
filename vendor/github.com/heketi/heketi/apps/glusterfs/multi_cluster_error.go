//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package glusterfs

import (
	"fmt"
	"strings"
)

type ClusterErrorMap map[string]error

type MultiClusterError struct {
	prefix string
	errors ClusterErrorMap
}

// Add an error originating with cluster `c` to the captured
// errors map.
func (m ClusterErrorMap) Add(c string, e error) {
	m[c] = e
}

// ToError returns either an error or nil depending on the number of errors in
// the map. It returns nil if no errors were captured. It returns a new
// MultiClusterError if more than one error was captured. It returns the
// original error if only one error was captured.
func (m ClusterErrorMap) ToError(prefix string) error {
	switch len(m) {
	case 0:
		return nil
	case 1:
		for _, err := range m {
			return err
		}
	}
	return NewMultiClusterError(prefix, m)
}

// NewMultiClusterError returns a MultiClusterError with the given
// prefix text. Prefix text will be used in the error string if
// more than one error is captured.
func NewMultiClusterError(p string, m ClusterErrorMap) *MultiClusterError {
	return &MultiClusterError{
		prefix: p,
		errors: m,
	}
}

// Error returns the error string for the multi cluster error.
// If only one error was captured, it returns the text of that
// error alone. If more than one error was captured, it returns
// formatted text containing all captured errors.
func (m *MultiClusterError) Error() string {
	if len(m.errors) == 0 {
		return "(no errors)"
	}
	if len(m.errors) == 1 {
		for _, v := range m.errors {
			return v.Error()
		}
	}
	errs := []string{}
	if m.prefix != "" {
		errs = append(errs, m.prefix)
	}
	for k, v := range m.errors {
		errs = append(errs, fmt.Sprintf("Cluster %v: %v", k, v.Error()))
	}
	return strings.Join(errs, "\n")
}
