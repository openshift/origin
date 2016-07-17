package util

import (
	"fmt"
	"runtime/debug"

	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/diagnostics/log"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// DetermineRequestedDiagnostics determines which diagnostic the user wants to run
// returns error or diagnostic names
func DetermineRequestedDiagnostics(available []string, requested []string, logger *log.Logger) (error, []string) {
	diagnostics := []string{}

	if len(requested) == 0 { // not specified, use the available list
		diagnostics = available
	} else if diagnostics = sets.NewString(requested...).Intersection(sets.NewString(available...)).List(); len(diagnostics) == 0 {
		logger.Error("CED6001", log.EvalTemplate("CED6001", "None of the requested diagnostics are available:\n  {{.requested}}\nPlease try from the following:\n  {{.available}}",
			log.Hash{"requested": requested, "available": available}))
		return fmt.Errorf("No requested diagnostics available"), diagnostics
	} else if len(diagnostics) < len(requested) {
		logger.Error("CED6002", log.EvalTemplate("CED6002", "Of the requested diagnostics:\n    {{.requested}}\nonly these are available:\n    {{.diagnostics}}\nThe list of all possible is:\n    {{.available}}",
			log.Hash{"requested": requested, "diagnostics": diagnostics, "available": available}))
		return fmt.Errorf("Not all requested diagnostics are available"), diagnostics
	}

	return nil, diagnostics
}

// RunDiagnostics performs the actual execution of diagnostics once they're built.
func RunDiagnostics(logger *log.Logger, diagnostics []types.Diagnostic, warnCount int, errorCount int) (bool, error, int, int) {
	for _, diagnostic := range diagnostics {
		func() { // wrap diagnostic panic nicely in case of developer error
			defer func() {
				if r := recover(); r != nil {
					errorCount += 1
					stack := debug.Stack()
					logger.Error("CED7001",
						fmt.Sprintf("While running the %s diagnostic, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%s\n%s",
							diagnostic.Name(), fmt.Sprintf("%v", r), stack))
				}
			}()

			if canRun, reason := diagnostic.CanRun(); !canRun {
				if reason == nil {
					logger.Notice("CED7002", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
				} else {
					logger.Notice("CED7003", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s\nBecause: %s", diagnostic.Name(), diagnostic.Description(), reason.Error()))
				}
				return
			}

			logger.Notice("CED7004", fmt.Sprintf("Running diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
			r := diagnostic.Check()
			for _, entry := range r.Logs() {
				logger.LogEntry(entry)
			}
			warnCount += len(r.Warnings())
			errorCount += len(r.Errors())
		}()
	}

	return errorCount > 0, nil, warnCount, errorCount
}
