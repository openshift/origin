package in_pod

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/options"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/util"
)

// PodDiagnosticsOptions holds values received from environment variables
// for the command to operate.
type PodDiagnosticsOptions struct {
	// list of diagnostic names to limit what is run
	RequestedDiagnostics []string
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The Logger is built with the options and should be used for all diagnostic output.
	logger *log.Logger
}

// returns the logger built according to options (must be Complete()ed)
func (o *PodDiagnosticsOptions) Logger() *log.Logger {
	return o.logger
}

const (
	InPodDiagnosticRecommendedName = "inpod-poddiagnostic"

	// Standard locations for the secrets mounted in pods
	StandardMasterCaPath = "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt"
	StandardTokenPath    = "/var/run/secrets/kubernetes.io/serviceaccount/token"
	StandardMasterUrl    = "https://kubernetes.default.svc.cluster.local"
)

var longPodDiagDescription = templates.LongDesc(`
	This utility is intended to run diagnostics inside a container and
	log the results so that the calling diagnostic can report them.`)

// NewCommandPodDiagnostics is the command for running pod diagnostics.
func NewCommandPodDiagnostics(name string, out io.Writer) *cobra.Command {
	o := &PodDiagnosticsOptions{
		RequestedDiagnostics: []string{},
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:    name,
		Short:  "Within a pod, run pod diagnostics",
		Long:   fmt.Sprintf(longPodDiagDescription),
		Run:    util.CommandRunFunc(o),
		Hidden: true,
	}
	cmd.SetOutput(out) // for output re: usage / help

	options.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, options.RecommendedLoggerOptionFlags())

	return cmd
}

// Complete fills in PodDiagnosticsOptions needed if the command is actually invoked.
func (o *PodDiagnosticsOptions) Complete(c *cobra.Command, args []string) error {
	var err error
	o.logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	o.RequestedDiagnostics = append(o.RequestedDiagnostics, args...)
	if len(o.RequestedDiagnostics) == 0 {
		o.RequestedDiagnostics = availablePodDiagnostics.List()
	}

	return nil
}

// RunDiagnostics builds diagnostics based on the options and executes them, returning fatal error(s) only.
func (o PodDiagnosticsOptions) RunDiagnostics() error {
	var fatal error
	var diagnostics []types.Diagnostic

	func() { // don't trust discovery/build of diagnostics; wrap panic nicely in case of developer error
		defer func() {
			if r := recover(); r != nil {
				fatal = fmt.Errorf("While building the diagnostics, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%v\n%s", r, debug.Stack())
			}
		}() // deferred panic handler

		diagnostics, fatal = o.buildPodDiagnostics()
	}()

	if fatal != nil {
		return fatal
	}

	return util.RunDiagnostics(o.Logger(), diagnostics)
}

var (
	// availablePodDiagnostics contains the names of host diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availablePodDiagnostics = sets.NewString(PodCheckDnsName, PodCheckAuthName)
)

// buildPodDiagnostics builds host Diagnostic objects based on the host environment.
// Returns the Diagnostics built, and any fatal error encountered during the building of diagnostics.
func (o PodDiagnosticsOptions) buildPodDiagnostics() ([]types.Diagnostic, error) {
	diagnostics := []types.Diagnostic{}
	err, requestedDiagnostics := util.DetermineRequestedDiagnostics(availablePodDiagnostics.List(), o.RequestedDiagnostics, o.Logger())
	if err != nil {
		return diagnostics, err // don't waste time on discovery
	}
	// TODO: check we're actually in a container

	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {

		case PodCheckDnsName:
			diagnostics = append(diagnostics, PodCheckDns{})

		case PodCheckAuthName:
			diagnostics = append(diagnostics, PodCheckAuth{
				MasterCaPath: StandardMasterCaPath,
				TokenPath:    StandardTokenPath,
				MasterUrl:    StandardMasterUrl,
			})

		default:
			return diagnostics, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	return diagnostics, nil
}
