package raven

import (
	"reflect"
	"regexp"
)

var errorMsgPattern = regexp.MustCompile(`\A(\w+): (.+)\z`)

// NewException constructs an Exception using provided Error and Stacktrace
func NewException(err error, stacktrace *Stacktrace) *Exception {
	msg := err.Error()
	ex := &Exception{
		Stacktrace: stacktrace,
		Value:      msg,
		Type:       reflect.TypeOf(err).String(),
	}
	if m := errorMsgPattern.FindStringSubmatch(msg); m != nil {
		ex.Module, ex.Value = m[1], m[2]
	}
	return ex
}

// Exception defines Sentry's spec compliant interface holding Exception information - https://docs.sentry.io/development/sdk-dev/interfaces/exception/
type Exception struct {
	// Required
	Value string `json:"value"`

	// Optional
	Type       string      `json:"type,omitempty"`
	Module     string      `json:"module,omitempty"`
	Stacktrace *Stacktrace `json:"stacktrace,omitempty"`
}

// Class provides name of implemented Sentry's interface
func (e *Exception) Class() string { return "exception" }

// Culprit tries to read top-most error message from Exception's stacktrace
func (e *Exception) Culprit() string {
	if e.Stacktrace == nil {
		return ""
	}
	return e.Stacktrace.Culprit()
}

// Exceptions defines Sentry's spec compliant interface holding Exceptions information - https://docs.sentry.io/development/sdk-dev/interfaces/exception/
type Exceptions struct {
	// Required
	Values []*Exception `json:"values"`
}

// Class provides name of implemented Sentry's interface
func (es Exceptions) Class() string { return "exception" }
