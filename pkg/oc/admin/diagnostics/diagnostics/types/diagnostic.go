package types

import (
	"fmt"
	"runtime"
	"strings"
	"sync"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/util/sets"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
)

// Diagnostic provides the interface for building diagnostics that can execute as part of the diagnostic framework.
// The Name and Description methods are used to identify which diagnostic is running in the output.
// Requirements() identifies the common parameters this diagnostic may require.
// The CanRun() method provides a pre-execution check for whether the diagnostic is relevant and runnable as constructed.
// If not, a user-facing reason for skipping the diagnostic can be given.
// Finally, the Check() method runs the diagnostic with the resulting messages and errors returned in a result object.
// It should be assumed a Diagnostic can run in parallel with other Diagnostics.
type Diagnostic interface {
	Name() string
	Description() string
	Requirements() (client bool, host bool)
	CanRun() (canRun bool, reason error)
	Check() DiagnosticResult
}

// DiagnosticsList is a simple list type for providing the Names() method
type DiagnosticList []Diagnostic

// Names returns a set of the names of the diagnostics in the list
func (d DiagnosticList) Names() sets.String {
	names := sets.NewString()
	for _, diag := range d {
		names.Insert(diag.Name())
	}
	return names
}

// Diagnostic provides an interface for finishing initialization of a diagnostic.
type IncompleteDiagnostic interface {
	// Complete runs just before CanRun; it can log issues to the logger. Returns error on misconfiguration.
	Complete(*log.Logger) error
}

// Parameter is used by an individual diagnostic to specify non-shared parameters for itself
// Name is a lowercase string that will be used to generate a CLI flag
// Description is used to describe the same flag
// Target is a pointer to what the flag should fill in
// Default is the default value for the flag description
type Parameter struct {
	Name        string
	Description string
	Target      interface{}
	Default     interface{}
}

// ParameterizedDiagnostic is a Diagnostic that can accept arbitrary parameters specifically for it.
// AvailableParameters is used to describe or validate the parameters given on the command line.
type ParameterizedDiagnostic interface {
	Diagnostic
	AvailableParameters() []Parameter
}

// ParameterizedDiagnosticMap holds PDs by name for later lookup
type ParameterizedDiagnosticMap map[string]ParameterizedDiagnostic

// NewParameterizedDiagnosticMap filters PDs from a list of diagnostics into a PDMap.
func NewParameterizedDiagnosticMap(diags ...Diagnostic) ParameterizedDiagnosticMap {
	m := ParameterizedDiagnosticMap{}
	for _, diag := range diags {
		if pd, ok := diag.(ParameterizedDiagnostic); ok {
			m[diag.Name()] = pd
		}
	}
	return m
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
	lock     sync.Mutex
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
	r.lock.Lock()
	defer r.lock.Unlock()
	return r.failure
}

func (r *diagnosticResultImpl) Logs() []log.Entry {
	r.lock.Lock()
	defer r.lock.Unlock()
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
	r.lock.Lock()
	defer r.lock.Unlock()
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
	r.lock.Lock()
	defer r.lock.Unlock()
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
	r.appendLogs(2, log.Entry{ID: id, Origin: r.caller(2), Level: log.ErrorLevel, Message: msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendErrors(de)
	} else {
		r.appendErrors(DiagnosticError{id, msg, err})
	}
}
func (r *diagnosticResultImpl) logWarning(id string, err error, msg string) {
	r.appendLogs(2, log.Entry{ID: id, Origin: r.caller(2), Level: log.WarnLevel, Message: msg})
	if de, ok := err.(DiagnosticError); ok {
		r.appendWarnings(de)
	} else {
		r.appendWarnings(DiagnosticError{id, msg, err})
	}
}
func (r *diagnosticResultImpl) logMessage(id string, level log.Level, msg string) {
	r.appendLogs(2, log.Entry{ID: id, Origin: r.caller(2), Level: level, Message: msg})
}

// Public ingress functions
// Errors are recorded in the result as errors plus log entries
func (r *diagnosticResultImpl) Error(id string, err error, text string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logError(id, err, text)
}

// Warnings are recorded in the result as warnings plus log entries
func (r *diagnosticResultImpl) Warn(id string, err error, text string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logWarning(id, err, text)
}

// Info/Debug are just recorded as log entries.
func (r *diagnosticResultImpl) Info(id string, text string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logMessage(id, log.InfoLevel, text)
}
func (r *diagnosticResultImpl) Debug(id string, text string) {
	r.lock.Lock()
	defer r.lock.Unlock()
	r.logMessage(id, log.DebugLevel, text)
}
