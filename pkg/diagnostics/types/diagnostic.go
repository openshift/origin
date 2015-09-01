package types

import (
	"fmt"
	"github.com/golang/glog"
	"runtime"
	"strings"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

// Diagnostic provides the interface for building diagnostics that can execute as part of the diagnostic framework.
// The Name and Description methods are used to identify which diagnostic is running in the output.
// The CanRun() method provides a pre-execution check for whether the diagnostic is relevant and runnable as constructed.
// If not, a user-facing reason for skipping the diagnostic can be given.
// Finally, the Check() method runs the diagnostic with the resulting messages and errors returned in a result object.
// It should be assumed a Diagnostic can run in parallel with other Diagnostics.
type Diagnostic interface {
	Name() string
	Description() string
	CanRun() (canRun bool, reason error)
	Check() DiagnosticResult
}

// DiagnosticResult provides a result object for diagnostics, accumulating the messages and errors
// that the diagnostic generates as it runs.
type DiagnosticResult interface {
	// Failure is true if there are any errors entered.
	Failure() bool
	// Logs/Warnings/Errors entered into the result object
	Logs() []log.Entry
	Warnings() []DiagnosticError
	Errors() []DiagnosticError
	// <Level> just takes a plain string, no formatting
	// <Level>f provides format string params
	// <Level>t interface{} should be a log.Hash for a template
	// Error and Warning add an entry to both logs and corresponding list of errors/warnings.
	Error(id string, err error, text string)
	Warn(id string, err error, text string)
	Info(id string, text string)
	Debug(id string, text string)
}

type diagnosticResultImpl struct {
	failure  bool
	origin   string // origin of the results, usually the diagnostic name; included in log Entries
	logs     []log.Entry
	warnings []DiagnosticError
	errors   []DiagnosticError
}

// NewDiagnosticResult generates an internally-implemented DiagnosticResult.
// The origin may be output with some log messages to help identify where in code it originated.
func NewDiagnosticResult(origin string) DiagnosticResult {
	return &diagnosticResultImpl{
		origin:   origin,
		errors:   []DiagnosticError{},
		warnings: []DiagnosticError{},
		logs:     []log.Entry{},
	}
}

func (r *diagnosticResultImpl) appendLogs(stackDepth int, entry ...log.Entry) {
	if r.logs == nil {
		r.logs = make([]log.Entry, 0)
	}
	r.logs = append(r.logs, entry...)
	// glog immediately for debugging when a diagnostic silently chokes
	for _, entry := range entry {
		if glog.V(glog.Level(6 - entry.Level.Level)) {
			glog.InfoDepth(stackDepth, entry.Message)
		}
	}
}

func (r *diagnosticResultImpl) Failure() bool {
	return r.failure
}

func (r *diagnosticResultImpl) Logs() []log.Entry {
	if r.logs == nil {
		return make([]log.Entry, 0)
	}
	return r.logs
}

func (r *diagnosticResultImpl) appendWarnings(warn ...DiagnosticError) {
	if r.warnings == nil {
		r.warnings = make([]DiagnosticError, 0)
	}
	r.warnings = append(r.warnings, warn...)
}

func (r *diagnosticResultImpl) Warnings() []DiagnosticError {
	if r.warnings == nil {
		return make([]DiagnosticError, 0)
	}
	return r.warnings
}

func (r *diagnosticResultImpl) appendErrors(err ...DiagnosticError) {
	if r.errors == nil {
		r.errors = make([]DiagnosticError, 0)
	}
	r.failure = true
	r.errors = append(r.errors, err...)
}

func (r *diagnosticResultImpl) Errors() []DiagnosticError {
	if r.errors == nil {
		return make([]DiagnosticError, 0)
	}
	return r.errors
}

// basic ingress functions (private)
func (r *diagnosticResultImpl) caller(depth int) string {
	if _, file, line, ok := runtime.Caller(depth + 1); ok {
		paths := strings.SplitAfter(file, "github.com/")
		return fmt.Sprintf("diagnostic %s@%s:%d", r.origin, paths[len(paths)-1], line)
	}
	return "diagnostic " + r.origin
}
func (r *diagnosticResultImpl) logError(id string, err error, msg string) {
	r.appendLogs(2, log.Entry{id, r.caller(2), log.ErrorLevel, msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendErrors(de)
	} else {
		r.appendErrors(DiagnosticError{id, msg, err})
	}
}
func (r *diagnosticResultImpl) logWarning(id string, err error, msg string) {
	r.appendLogs(2, log.Entry{id, r.caller(2), log.WarnLevel, msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendWarnings(de)
	} else {
		r.appendWarnings(DiagnosticError{id, msg, err})
	}
}
func (r *diagnosticResultImpl) logMessage(id string, level log.Level, msg string) {
	r.appendLogs(2, log.Entry{id, r.caller(2), level, msg})
}

// Public ingress functions
// Errors are recorded in the result as errors plus log entries
func (r *diagnosticResultImpl) Error(id string, err error, text string) {
	r.logError(id, err, text)
}

// Warnings are recorded in the result as warnings plus log entries
func (r *diagnosticResultImpl) Warn(id string, err error, text string) {
	r.logWarning(id, err, text)
}

// Info/Debug are just recorded as log entries.
func (r *diagnosticResultImpl) Info(id string, text string) {
	r.logMessage(id, log.InfoLevel, text)
}
func (r *diagnosticResultImpl) Debug(id string, text string) {
	r.logMessage(id, log.DebugLevel, text)
}
