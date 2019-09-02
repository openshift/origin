// Copyright ©2019 The Gonum Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// +build gofuzz

package fuzz

import (
	"bytes"
	"os/exec"

	"gonum.org/v1/gonum/graph/formats/dot"
)

// Fuzz implements the fuzzing function required for go-fuzz.
//
// See documentation at https://github.com/dvyukov/go-fuzz.
func Fuzz(data []byte) int {
	// We don't accept empty data; the dot command does.
	if len(data) == 0 || bytes.Equal(data, []byte{0}) {
		return -1
	}

	// Check that dot accepts the input without complaint.
	cmd := exec.Command("dot")
	cmd.Stdin = bytes.NewReader(data)
	err := cmd.Run()
	if err != nil {
		return 0
	}

	// Try to parse the data.
	_, err = dot.Parse(bytes.NewReader(data))
	if err != nil {
		panic("could not parse good dot")
	}
	return 1
}
