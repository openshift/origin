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
	"path/filepath"
	"runtime"
)

// TraceSkip is like Trace, but additionally skips stack frames. So TaceSkip(0)
// behaves like Trace(), TraceSkip(1) refers to the call of the current
// function, TraceSkip(2) refers to the call of the calling function, and so on.
func TraceSkip(skip int) (string, string, int) {
	pc := make([]uintptr, 15)
	n := runtime.Callers(skip+2, pc)
	frames := runtime.CallersFrames(pc[:n])
	frame, _ := frames.Next()
	return frame.Function, frame.File, frame.Line
}

// Trace returns the function name, the file name and the line number of the
// place where Trace() is called.
func Trace() (funcName string, fileName string, lineNr int) {
	return TraceSkip(1)
}

// TraceFunc is a convenience function that just returns the bare function name
// without path of the function from where TraceFunc() is called.
func TraceFunc() (funcName string) {
	fun, _, _ := TraceSkip(1)
	return filepath.Base(fun)
}
