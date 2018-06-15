package glog

import (
	"fmt"
	"io"
	"strings"

	"github.com/golang/glog"
)

// Logger is a simple interface that is roughly equivalent to glog.
type Logger interface {
	Is(level int) bool
	V(level int) Logger
	Infof(format string, args ...interface{})
}

// ToFile creates a logger that will log any items at level or below to file, and defer
// any other output to glog (no matter what the level is.)
func ToFile(w io.Writer, level int) Logger {
	return file{w, level}
}

var (
	// None implements the Logger interface but does nothing with the log output
	None Logger = discard{}
	// Log implements the Logger interface for Glog
	Log Logger = glogger{}
)

// discard is a Logger that outputs nothing.
type discard struct{}

func (discard) Is(level int) bool                { return false }
func (discard) V(level int) Logger               { return None }
func (discard) Infof(_ string, _ ...interface{}) {}

// glogger outputs log messages to glog
type glogger struct{}

func (glogger) Is(level int) bool {
	return bool(glog.V(glog.Level(level)))
}

func (glogger) V(level int) Logger {
	return gverbose{glog.V(glog.Level(level))}
}

func (glogger) Infof(format string, args ...interface{}) {
	glog.InfoDepth(2, fmt.Sprintf(format, args...))
}

// gverbose handles glog.V(x) calls
type gverbose struct {
	glog.Verbose
}

func (gverbose) Is(level int) bool {
	return bool(glog.V(glog.Level(level)))
}

func (gverbose) V(level int) Logger {
	if glog.V(glog.Level(level)) {
		return Log
	}
	return None
}

func (g gverbose) Infof(format string, args ...interface{}) {
	if g.Verbose {
		glog.InfoDepth(2, fmt.Sprintf(format, args...))
	}
}

// file logs the provided messages at level or below to the writer, or delegates
// to glog.
type file struct {
	w     io.Writer
	level int
}

func (f file) Is(level int) bool {
	return level <= f.level || bool(glog.V(glog.Level(level)))
}

func (f file) V(level int) Logger {
	// only log things that glog allows
	if !glog.V(glog.Level(level)) {
		return None
	}
	// send anything above our level to glog
	if level > f.level {
		return Log
	}
	return f
}

func (f file) Infof(format string, args ...interface{}) {
	fmt.Fprintf(f.w, format, args...)
	if !strings.HasSuffix(format, "\n") {
		fmt.Fprintln(f.w)
	}
}
