package diagnostics

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/admin/diagnostics/options"
	"github.com/openshift/origin/pkg/diagnostics/log"
	poddiag "github.com/openshift/origin/pkg/diagnostics/pod"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// PodDiagnosticsOptions holds values received from environment variables
// for the command to operate.
type PodDiagnosticsOptions struct {
	// list of diagnostic names to limit what is run
	RequestedDiagnostics []string
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The Logger is built with the options and should be used for all diagnostic output.
	Logger *log.Logger
}

const (
	// Standard locations for the secrets mounted in pods
	StandardMasterCaPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	StandardTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	StandardMasterUrl    = "https://kubernetes.default.svc.cluster.local"
)

const longPodDiagDescription = `
This utility is intended to run diagnostics inside a container and
log the results so that the calling diagnostic can report them.
`

// NewCommandPodDiagnostics is the command for running pod diagnostics.
func NewCommandPodDiagnostics(name string, out io.Writer) *cobra.Command {
	o := &PodDiagnosticsOptions{
		RequestedDiagnostics: []string{},
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Within a pod, run pod diagnostics",
		Long:  fmt.Sprintf(longPodDiagDescription),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(args))

			failed, err, warnCount, errorCount := o.BuildAndRunDiagnostics()
			o.Logger.Summary(warnCount, errorCount)

			kcmdutil.CheckErr(err)
			if failed {
				os.Exit(255)
			}

		},
	}
	cmd.SetOutput(out) // for output re: usage / help

	options.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, options.RecommendedLoggerOptionFlags())

	return cmd
}

// Complete fills in PodDiagnosticsOptions needed if the command is actually invoked.
func (o *PodDiagnosticsOptions) Complete(args []string) error {
	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	o.RequestedDiagnostics = append(o.RequestedDiagnostics, args...)
	if len(o.RequestedDiagnostics) == 0 {
		o.RequestedDiagnostics = availablePodDiagnostics.List()
	}

	return nil
}

// BuildAndRunDiagnostics builds diagnostics based on the options and executes them, returning a summary.
func (o PodDiagnosticsOptions) BuildAndRunDiagnostics() (bool, error, int, int) {
	failed := false
	errors := []error{}
	diagnostics := []types.Diagnostic{}

	func() { // don't trust discovery/build of diagnostics; wrap panic nicely in case of developer error
		defer func() {
			if r := recover(); r != nil {
				failed = true
				stack := debug.Stack()
				errors = append(errors, fmt.Errorf("While building the diagnostics, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%v\n%s", r, stack))
			}
		}() // deferred panic handler
		podDiags, ok, err := o.buildPodDiagnostics()
		failed = failed || !ok
		if ok {
			diagnostics = append(diagnostics, podDiags...)
		}
		if err != nil {
			errors = append(errors, err...)
		}

	}()

	if failed {
		return failed, kutilerrors.NewAggregate(errors), 0, len(errors)
	}

	failed, err, numWarnings, numErrors := runDiagnostics(o.Logger, diagnostics, 0, len(errors))
	return failed, err, numWarnings, numErrors
}

var (
	// availablePodDiagnostics contains the names of host diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availablePodDiagnostics = sets.NewString(poddiag.PodCheckDnsName, poddiag.PodCheckAuthName)
)

// buildPodDiagnostics builds host Diagnostic objects based on the host environment.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.
func (o PodDiagnosticsOptions) buildPodDiagnostics() ([]types.Diagnostic, bool, []error) {
	diagnostics := []types.Diagnostic{}
	err, requestedDiagnostics := determineRequestedDiagnostics(availablePodDiagnostics.List(), o.RequestedDiagnostics, o.Logger)
	if err != nil {
		return diagnostics, false, []error{err} // don't waste time on discovery
	}
	// TODO: check we're actually in a container

	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {

		case poddiag.PodCheckDnsName:
			diagnostics = append(diagnostics, poddiag.PodCheckDns{})

		case poddiag.PodCheckAuthName:
			diagnostics = append(diagnostics, poddiag.PodCheckAuth{
				MasterCaPath: StandardMasterCaPath,
				TokenPath:    StandardTokenPath,
				MasterUrl:    StandardMasterUrl,
			})

		default:
			return diagnostics, false, []error{fmt.Errorf("unknown diagnostic: %v", diagnosticName)}
		}
	}

	return diagnostics, true, nil
}

// runDiagnostics performs the actual execution of diagnostics once they're built.
func runDiagnostics(logger *log.Logger, diagnostics []types.Diagnostic, warnCount int, errorCount int) (bool, error, int, int) {
	for _, diagnostic := range diagnostics {
		func() { // wrap diagnostic panic nicely in case of developer error
			defer func() {
				if r := recover(); r != nil {
					errorCount += 1
					stack := debug.Stack()
					logger.Error("CED5001",
						fmt.Sprintf("While running the %s diagnostic, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%s\n%s",
							diagnostic.Name(), fmt.Sprintf("%v", r), stack))
				}
			}()

			if canRun, reason := diagnostic.CanRun(); !canRun {
				if reason == nil {
					logger.Notice("CED5002", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
				} else {
					logger.Notice("CED5003", fmt.Sprintf("Skipping diagnostic: %s\nDescription: %s\nBecause: %s", diagnostic.Name(), diagnostic.Description(), reason.Error()))
				}
				return
			}

			logger.Notice("CED5004", fmt.Sprintf("Running diagnostic: %s\nDescription: %s", diagnostic.Name(), diagnostic.Description()))
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

// determineRequestedDiagnostics determines which diagnostic the user wants to run
// based on the -d list and the available diagnostics.
// returns error or diagnostic names
func determineRequestedDiagnostics(available []string, requested []string, logger *log.Logger) (error, []string) {
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
	} // else it's a valid list.
	return nil, diagnostics
}
