//
// Copyright (c) 2018 The heketi Authors
//
// This file is licensed to you under your choice of the GNU Lesser
// General Public License, version 3 or any later version (LGPLv3 or
// later), or the GNU General Public License, version 2 (GPLv2), in all
// cases as published by the Free Software Foundation.
//

package logging

import (
	"strings"
	"testing"

	"github.com/heketi/tests"
)

func TestTrace(t *testing.T) {
	fun, file, line := Trace()
	_, _, nextLine := Trace()
	tests.Assert(t, strings.HasSuffix(fun, "logging.TestTrace"))
	tests.Assert(t, strings.HasSuffix(file, "trace_test.go"))
	tests.Assert(t, nextLine-line == 1)
}

func TestTraceSkip(t *testing.T) {
	var fun string
	func() { func() { fun, _, _ = TraceSkip(2) }() }()
	tests.Assert(t, strings.HasSuffix(fun, "logging.TestTraceSkip"))
}

func TestTraceFunc(t *testing.T) {
	fun := TraceFunc()
	tests.Assert(t, fun == "logging.TestTraceFunc")
}
