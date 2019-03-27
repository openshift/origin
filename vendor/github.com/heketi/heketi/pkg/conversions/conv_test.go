//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package conversions

import (
	"testing"

	"github.com/heketi/tests"
)

func TestBoolToYNTrue(t *testing.T) {
	result := BoolToYN(true)
	tests.Assert(t, result == "y", "calling BoolToYN(true), expected", "y", "got", result)
}

func TestBoolToYNFalse(t *testing.T) {
	result := BoolToYN(false)
	tests.Assert(t, result == "n", "calling BoolToYN(false), expected", "n", "got", result)
}
