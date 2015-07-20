package types

import (
	"fmt"
	"github.com/golang/glog"
	"runtime"
	"strings"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

type Diagnostic interface {
	Name() string
	Description() string
	CanRun() (canRun bool, reason error)
	Check() *DiagnosticResult
}

type DiagnosticResult struct {
	failure  bool
	origin   string // name of diagnostic; automatically inserted into log Entries
	logs     []log.Entry
	warnings []DiagnosticError
	errors   []DiagnosticError
}

func NewDiagnosticResult(origin string) *DiagnosticResult {
	return &DiagnosticResult{origin: origin}
}

func (r *DiagnosticResult) Complete() *DiagnosticResult {
	if r.errors == nil {
		r.errors = make([]DiagnosticError, 0)
	}
	if r.warnings == nil {
		r.warnings = make([]DiagnosticError, 0)
	}
	if r.logs == nil {
		r.logs = make([]log.Entry, 0)
	}
	return r
}

func (r *DiagnosticResult) appendLogs(stackDepth int, entry ...log.Entry) {
	if r.logs == nil {
		r.logs = make([]log.Entry, 0)
	}
	r.logs = append(r.logs, entry...)
	// glog immediately for debugging when a diagnostic silently chokes
	for _, entry := range entry {
		if glog.V(glog.Level(6 - entry.Level.Level)) {
			glog.InfoDepth(stackDepth, entry.Message.String())
		}
	}
}

func (r *DiagnosticResult) Failure() bool {
	return r.failure
}

func (r *DiagnosticResult) Logs() []log.Entry {
	if r.logs == nil {
		return make([]log.Entry, 0)
	}
	return r.logs
}

func (r *DiagnosticResult) appendWarnings(warn ...DiagnosticError) {
	if r.warnings == nil {
		r.warnings = make([]DiagnosticError, 0)
	}
	r.warnings = append(r.warnings, warn...)
}

func (r *DiagnosticResult) Warnings() []DiagnosticError {
	if r.warnings == nil {
		return make([]DiagnosticError, 0)
	}
	return r.warnings
}

func (r *DiagnosticResult) appendErrors(err ...DiagnosticError) {
	if r.errors == nil {
		r.errors = make([]DiagnosticError, 0)
	}
	r.failure = true
	r.errors = append(r.errors, err...)
}

func (r *DiagnosticResult) Errors() []DiagnosticError {
	if r.errors == nil {
		return make([]DiagnosticError, 0)
	}
	return r.errors
}

func (r *DiagnosticResult) Append(r2 *DiagnosticResult) {
	r.Complete()
	r2.Complete()
	r.logs = append(r.logs, r2.logs...)
	r.warnings = append(r.warnings, r2.warnings...)
	r.errors = append(r.errors, r2.errors...)
	r.failure = r.failure || r2.failure
}

// basic ingress functions (private)
func (r *DiagnosticResult) caller(depth int) string {
	if _, file, line, ok := runtime.Caller(depth + 1); ok {
		paths := strings.SplitAfter(file, "github.com/")
		return fmt.Sprintf("diagnostic %s@%s:%d", r.origin, paths[len(paths)-1], line)
	}
	return "diagnostic " + r.origin
}
func (r *DiagnosticResult) logError(id string, err error, msg *log.Message) {
	r.appendLogs(2, log.Entry{id, r.caller(2), log.ErrorLevel, *msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendErrors(de)
	} else {
		r.appendErrors(DiagnosticError{id, msg, err})
	}
}
func (r *DiagnosticResult) logWarning(id string, err error, msg *log.Message) {
	r.appendLogs(2, log.Entry{id, r.caller(2), log.WarnLevel, *msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendWarnings(de)
	} else {
		r.appendWarnings(DiagnosticError{id, msg, err})
	}
}
func (r *DiagnosticResult) logMessage(id string, level log.Level, msg *log.Message) {
	r.appendLogs(2, log.Entry{id, r.caller(2), level, *msg})
}

// Public ingress functions
// Errors are recorded as errors and also logged
func (r *DiagnosticResult) Error(id string, err error, text string) {
	r.logError(id, err, &log.Message{id, "", nil, text})
}
func (r *DiagnosticResult) Errorf(id string, err error, format string, a ...interface{}) {
	r.logError(id, err, &log.Message{id, "", nil, fmt.Sprintf(format, a...)})
}
func (r *DiagnosticResult) Errort(id string, err error, template string, data interface{} /* log.Hash */) {
	r.logError(id, err, &log.Message{id, template, data, ""})
}

// Warnings are recorded as warnings and also logged
func (r *DiagnosticResult) Warn(id string, err error, text string) {
	r.logWarning(id, err, &log.Message{id, "", nil, text})
}
func (r *DiagnosticResult) Warnf(id string, err error, format string, a ...interface{}) {
	r.logWarning(id, err, &log.Message{id, "", nil, fmt.Sprintf(format, a...)})
}
func (r *DiagnosticResult) Warnt(id string, err error, template string, data interface{} /* log.Hash */) {
	r.logWarning(id, err, &log.Message{id, template, data, ""})
}

// Info/Debug are just logged.
func (r *DiagnosticResult) Info(id string, text string) {
	r.logMessage(id, log.InfoLevel, &log.Message{id, "", nil, text})
}
func (r *DiagnosticResult) Infof(id string, format string, a ...interface{}) {
	r.logMessage(id, log.InfoLevel, &log.Message{id, "", nil, fmt.Sprintf(format, a...)})
}
func (r *DiagnosticResult) Infot(id string, template string, data interface{} /* log.Hash */) {
	r.logMessage(id, log.InfoLevel, &log.Message{id, template, data, ""})
}
func (r *DiagnosticResult) Debug(id string, text string) {
	r.logMessage(id, log.DebugLevel, &log.Message{id, "", nil, text})
}
func (r *DiagnosticResult) Debugf(id string, format string, a ...interface{}) {
	r.logMessage(id, log.DebugLevel, &log.Message{id, "", nil, fmt.Sprintf(format, a...)})
}
func (r *DiagnosticResult) Debugt(id string, template string, data interface{} /* log.Hash */) {
	r.logMessage(id, log.DebugLevel, &log.Message{id, template, data, ""})
}
