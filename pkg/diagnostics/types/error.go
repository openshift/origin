package types

import (
	"fmt"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

type DiagnosticError struct {
	ID         string
	LogMessage *log.Message
	Cause      error
}

func (e DiagnosticError) Error() string {
	if e.LogMessage != nil {
		return fmt.Sprintf("%v", e.LogMessage)
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return e.ID
}

func IsDiagnosticError(e error) bool {
	_, ok := e.(DiagnosticError)
	return ok
}

// is the error a diagnostics error that matches the given ID?
func MatchesDiagError(err error, id string) bool {
	if derr, ok := err.(DiagnosticError); ok && derr.ID == id {
		return true
	}
	return false
}
