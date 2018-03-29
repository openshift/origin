package util

import (
	"fmt"
	"github.com/spf13/cobra"
	"os"
	"runtime/debug"

	"k8s.io/apimachinery/pkg/util/sets"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
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
func RunDiagnostics(logger *log.Logger, diagnostics []types.Diagnostic) error {
	runCount := 0
	for _, diagnostic := range diagnostics {
		func() { // wrap diagnostic panic nicely in case of developer error
			defer func() {
				if r := recover(); r != nil {
					logger.Error("CED7001",
						fmt.Sprintf("While running the %s diagnostic, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%s\n%s",
							diagnostic.Name(), fmt.Sprintf("%v", r), debug.Stack()))
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
			runCount += 1

			logger.Notice("CED7004", fmt.Sprintf("Running diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
			r := diagnostic.Check()
			for _, entry := range r.Logs() {
				logger.LogEntry(entry)
			}
		}()
	}

	if runCount == 0 {
		return fmt.Errorf("Requested diagnostic(s) skipped; nothing to run. See --help and consider setting flags or providing config to enable running.")
	}
	return nil
}

type OptionsRunner interface {
	Logger() *log.Logger
	Complete(*cobra.Command, []string) error
	RunDiagnostics() error
}

// returns a shared function that runs when one of these Commands actually executes
func CommandRunFunc(o OptionsRunner) func(c *cobra.Command, args []string) {
	return func(c *cobra.Command, args []string) {
		kcmdutil.CheckErr(o.Complete(c, args))

		if err := o.RunDiagnostics(); err != nil {
			o.Logger().Error("CED3001", fmt.Sprintf("Encountered fatal error while building diagnostics: %s", err.Error()))
		}
		o.Logger().Summary()
		if o.Logger().ErrorsSeen() {
			os.Exit(1)
		}
	}
}
