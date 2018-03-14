package in_pod

import (
	"fmt"
	"io"
	"runtime/debug"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/options"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/util"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const (
	InPodNetworkCheckRecommendedName = "inpod-networkcheck"
)

// NetworkPodDiagnosticsOptions holds values received from environment variables
// for the command to operate.
type NetworkPodDiagnosticsOptions struct {
	// list of diagnostic names to limit what is run
	RequestedDiagnostics []string
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The Logger is built with the options and should be used for all diagnostic output.
	logger *log.Logger
}

var longNetworkPodDiagDescription = templates.LongDesc(`
This utility is intended to run network diagnostics inside a privileged container and
log the results so that the calling diagnostic can report them.
`)

var (
	// availableNetworkPodDiagnostics contains the names of network diagnostics that can be executed
	// during a single run of diagnostics. Add more diagnostics to the list as they are defined.
	availableNetworkPodDiagnostics = sets.NewString(CheckNodeNetworkName, CheckPodNetworkName, CheckExternalNetworkName, CheckServiceNetworkName, CollectNetworkInfoName)
)

// NewCommandNetworkPodDiagnostics is the command for running network diagnostics.
func NewCommandNetworkPodDiagnostics(name string, out io.Writer) *cobra.Command {
	o := &NetworkPodDiagnosticsOptions{
		RequestedDiagnostics: []string{},
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:    name,
		Short:  "Within a privileged pod, run network diagnostics",
		Long:   fmt.Sprintf(longNetworkPodDiagDescription),
		Run:    util.CommandRunFunc(o),
		Hidden: true,
	}
	cmd.SetOutput(out) // for output re: usage / help

	options.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, options.RecommendedLoggerOptionFlags())

	return cmd
}

// Logger returns the logger built according to options (must be Complete()ed)
func (o *NetworkPodDiagnosticsOptions) Logger() *log.Logger {
	return o.logger
}

// Complete fills in NetworkPodDiagnosticsOptions needed if the command is actually invoked.
func (o *NetworkPodDiagnosticsOptions) Complete(c *cobra.Command, args []string) (err error) {
	o.logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	o.RequestedDiagnostics = append(o.RequestedDiagnostics, args...)
	if len(o.RequestedDiagnostics) == 0 {
		o.RequestedDiagnostics = availableNetworkPodDiagnostics.List()
	}

	return nil
}

// RunDiagnostics builds diagnostics based on the options and executes them, returning a summary.
func (o NetworkPodDiagnosticsOptions) RunDiagnostics() error {
	var fatal error
	diagnostics := []types.Diagnostic{}

	func() { // don't trust discovery/build of diagnostics; wrap panic nicely in case of developer error
		defer func() {
			if r := recover(); r != nil {
				fatal = fmt.Errorf("While building the diagnostics, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%v\n%s", r, debug.Stack())
			}
		}() // deferred panic handler

		diagnostics, fatal = o.buildNetworkPodDiagnostics()
	}()

	if fatal != nil {
		return fatal
	}

	return util.RunDiagnostics(o.Logger(), diagnostics)
}

// buildNetworkPodDiagnostics builds network Diagnostic objects based on the host environment.
// Returns the Diagnostics built or any fatal error encountered during the building of diagnostics.
func (o NetworkPodDiagnosticsOptions) buildNetworkPodDiagnostics() ([]types.Diagnostic, error) {
	diagnostics := []types.Diagnostic{}
	err, requestedDiagnostics := util.DetermineRequestedDiagnostics(availableNetworkPodDiagnostics.List(), o.RequestedDiagnostics, o.Logger())
	if err != nil {
		return nil, err // don't waste time on discovery
	}

	clientFlags := flag.NewFlagSet("client", flag.ContinueOnError) // hide the extensive set of client flags
	factory := osclientcmd.New(clientFlags)                        // that would otherwise be added to this command

	kubeClient, clientErr := factory.ClientSet()
	if clientErr != nil {
		return nil, clientErr
	}
	networkClient, err := factory.OpenshiftInternalNetworkClient()
	if err != nil {
		return nil, err
	}

	for _, diagnosticName := range requestedDiagnostics {
		switch diagnosticName {

		case CheckNodeNetworkName:
			diagnostics = append(diagnostics, CheckNodeNetwork{
				KubeClient: kubeClient,
			})

		case CheckPodNetworkName:
			diagnostics = append(diagnostics, CheckPodNetwork{
				KubeClient:           kubeClient,
				NetNamespacesClient:  networkClient.Network(),
				ClusterNetworkClient: networkClient.Network(),
			})

		case CheckExternalNetworkName:
			diagnostics = append(diagnostics, CheckExternalNetwork{})

		case CheckServiceNetworkName:
			diagnostics = append(diagnostics, CheckServiceNetwork{
				KubeClient:           kubeClient,
				NetNamespacesClient:  networkClient.Network(),
				ClusterNetworkClient: networkClient.Network(),
			})

		case CollectNetworkInfoName:
			diagnostics = append(diagnostics, CollectNetworkInfo{
				KubeClient: kubeClient,
			})

		default:
			return diagnostics, fmt.Errorf("unknown diagnostic: %v", diagnosticName)
		}
	}

	return diagnostics, nil
}
