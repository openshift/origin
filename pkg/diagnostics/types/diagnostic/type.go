package diagnostic

// This needed to be separate from other types to avoid import cycle
// diagnostic -> discovery -> types

import (
	"fmt"

	"github.com/openshift/origin/pkg/diagnostics/log"
)

type Diagnostic interface {
	Description() string
	CanRun() (canRun bool, reason error)
	Check() (success bool, info []log.Message, warnings []error, errors []error)
}

type DiagnosticError struct {
	ID          string
	Explanation string
	Cause       error

	LogMessage *log.Message
}

func NewDiagnosticError(id, explanation string, cause error) DiagnosticError {
	return DiagnosticError{id, explanation, cause, nil}
}

func NewDiagnosticErrorFromTemplate(id, template string, templateData interface{}) DiagnosticError {
	return DiagnosticError{id, "", nil,
		&log.Message{
			ID:           id,
			Template:     template,
			TemplateData: templateData,
		},
	}
}

func (e DiagnosticError) Error() string {
	if e.Cause != nil {
		return e.Cause.Error()
	}

	if e.LogMessage != nil {
		return fmt.Sprintf("%v", e.LogMessage)
	}

	return e.Explanation
}

func IsDiagnosticError(e error) bool {
	_, ok := e.(DiagnosticError)
	return ok
}
