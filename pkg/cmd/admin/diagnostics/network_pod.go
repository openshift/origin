package diagnostics

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kutilerrors "k8s.io/kubernetes/pkg/util/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/admin/diagnostics/options"
	"github.com/openshift/origin/pkg/cmd/admin/diagnostics/util"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/log"
	networkdiag "github.com/openshift/origin/pkg/diagnostics/networkpod"
	"github.com/openshift/origin/pkg/diagnostics/types"
)

// NetworkPodDiagnosticsOptions holds values received from environment variables
// for the command to operate.
type NetworkPodDiagnosticsOptions struct {
	// list of diagnostic names to limit what is run
	RequestedDiagnostics []string
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The Logger is built with the options and should be used for all diagnostic output.
	Logger *log.Logger
}

var longNetworkPodDiagDescription = templates.LongDesc(`
This utility is intended to run network diagnostics inside a privileged container and
log the results so that the calling diagnostic can report them.
`)

var (
	// availableNetworkPodDiagnostics contains the names of network diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableNetworkPodDiagnostics = sets.NewString(networkdiag.CheckNodeNetworkName, networkdiag.CheckPodNetworkName, networkdiag.CheckExternalNetworkName, networkdiag.CheckServiceNetworkName, networkdiag.CollectNetworkInfoName)
)

// NewCommandNetworkPodDiagnostics is the command for running network diagnostics.
func NewCommandNetworkPodDiagnostics(name string, out io.Writer) *cobra.Command {
	o := &NetworkPodDiagnosticsOptions{
		RequestedDiagnostics: []string{},
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Within a privileged pod, run network diagnostics",
		Long:  fmt.Sprintf(longNetworkPodDiagDescription),
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

// Complete fills in NetworkPodDiagnosticsOptions needed if the command is actually invoked.
func (o *NetworkPodDiagnosticsOptions) Complete(args []string) (err error) {
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	o.RequestedDiagnostics = append(o.RequestedDiagnostics, args...)
	if len(o.RequestedDiagnostics) == 0 {
		o.RequestedDiagnostics = availableNetworkPodDiagnostics.List()
	}

	return nil
}

// BuildAndRunDiagnostics builds diagnostics based on the options and executes them, returning a summary.
func (o NetworkPodDiagnosticsOptions) BuildAndRunDiagnostics() (failed bool, err error, numWarnings, numErrors int) {
	failed = false
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
		networkPodDiags, ok, err := o.buildNetworkPodDiagnostics()
		failed = failed || !ok
		if ok {
			diagnostics = append(diagnostics, networkPodDiags...)
		}
		if err != nil {
			errors = append(errors, err...)
		}
	}()

	if failed {
		return failed, kutilerrors.NewAggregate(errors), 0, len(errors)
	}

	failed, err, numWarnings, numErrors = util.RunDiagnostics(o.Logger, diagnostics, 0, len(errors))
	return failed, err, numWarnings, numErrors
}

// buildNetworkPodDiagnostics builds network Diagnostic objects based on the host environment.
// Returns the Diagnostics built, "ok" bool for whether to proceed or abort, and an error if any was encountered during the building of diagnostics.
func (o NetworkPodDiagnosticsOptions) buildNetworkPodDiagnostics() ([]types.Diagnostic, bool, []error) {
	diagnostics := []types.Diagnostic{}
	err, requestedDiagnostics := util.DetermineRequestedDiagnostics(availableNetworkPodDiagnostics.List(), o.RequestedDiagnostics, o.Logger)
	if err != nil {
		return diagnostics, false, []error{err} // don't waste time on discovery
	}

	clientFlags := flag.NewFlagSet("client", flag.ContinueOnError) // hide the extensive set of client flags
	factory := osclientcmd.New(clientFlags)                        // that would otherwise be added to this command

	osClient, kubeClient, clientErr := factory.Clients()
	if clientErr != nil {
		return diagnostics, false, []error{clientErr}
	}

	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {

		case networkdiag.CheckNodeNetworkName:
			diagnostics = append(diagnostics, networkdiag.CheckNodeNetwork{
				KubeClient: kubeClient,
			})

		case networkdiag.CheckPodNetworkName:
			diagnostics = append(diagnostics, networkdiag.CheckPodNetwork{
				KubeClient: kubeClient,
				OSClient:   osClient,
			})

		case networkdiag.CheckExternalNetworkName:
			diagnostics = append(diagnostics, networkdiag.CheckExternalNetwork{})

		case networkdiag.CheckServiceNetworkName:
			diagnostics = append(diagnostics, networkdiag.CheckServiceNetwork{
				KubeClient: kubeClient,
				OSClient:   osClient,
			})

		case networkdiag.CollectNetworkInfoName:
			diagnostics = append(diagnostics, networkdiag.CollectNetworkInfo{
				KubeClient: kubeClient,
			})

		default:
			return diagnostics, false, []error{fmt.Errorf("unknown diagnostic: %v", diagnosticName)}
		}
	}

	return diagnostics, true, nil
}
