package diagnostics

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"
	flag "github.com/spf13/pflag"

	kutilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"

	"github.com/openshift/origin/pkg/client/config"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	poddiag "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/client/pod/in_pod"
	networkpoddiag "github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/cluster/network/in_pod"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/log"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/diagnostics/types"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/options"
	"github.com/openshift/origin/pkg/oc/admin/diagnostics/util"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

// DiagnosticsOptions holds values received from command line flags as well as
// other objects generated for the command to operate.
type DiagnosticsOptions struct {
	// list of diagnostic name(s) to run
	RequestedDiagnostics sets.String
	// flag bindings for any diagnostics that require them
	ParameterizedDiagnostics types.ParameterizedDiagnosticMap

	// list available diagnostics and exit
	ListAll bool
	// specify locations of host config files
	MasterConfigLocation string
	NodeConfigLocation   string
	// indicate this is an openshift host despite lack of other indicators
	IsHost bool
	// When true, prevent diagnostics from changing API state (e.g. creating something)
	PreventModification bool
	// We need a factory for creating clients. Creating a factory
	// creates flags as a byproduct, most of which we don't want.
	// The command creates these and binds only the flags we want.
	ClientFlags *flag.FlagSet
	Factory     *osclientcmd.Factory
	// specify context name to be used for cluster-admin access
	ClientClusterContext string
	// LogOptions determine globally what the user wants to see and how.
	LogOptions *log.LoggerOptions
	// The logger is built with the options and should be used for all diagnostic output.
	logger *log.Logger
}

const (
	// Command name
	DiagnosticsRecommendedName    = "diagnostics"
	AllDiagnosticsRecommendedName = "all"

	// Standard locations for the host config files OpenShift uses.
	StandardMasterConfigPath string = "/etc/origin/master/master-config.yaml"
	StandardNodeConfigPath   string = "/etc/origin/node/node-config.yaml"
)

var (
	longDescription = templates.LongDesc(`
		This utility helps troubleshoot and diagnose known problems for an OpenShift cluster
		and/or local host. The base command runs a standard set of diagnostics:

		    %[1]s

		Available diagnostics vary based on client config and local OpenShift host config.
		Config files in standard locations for client, master, and node are used, or
		you may specify config files explicitly with flags. For example:

		    %[1]s --master-config=/etc/origin/master/master-config.yaml

		* Explicitly specifying a config file raises an error if it is not found.
		* A client config with cluster-admin access is required for most cluster diagnostics.
		* Diagnostics that require a config file are skipped if it is not found.
		* The standard set also skips diagnostics considered too heavyweight.

		An individual diagnostic may be run as a subcommand which may have flags
		for specifying options specific to that diagnostic.

		Finally, the "all" subcommand runs all available diagnostics (including heavyweight
		ones skipped in the standard set) and provides all individual diagnostic flags.
		`)
	longDescriptionAll = templates.LongDesc(`
		This utility helps troubleshoot and diagnose known problems for an OpenShift cluster
		and/or local host. This subcommand exists to run all available diagnostics:

		    %[1]s

		Available diagnostics vary based on client config and local OpenShift host config.
		All flags from the base command work similarly here, but all possible flags for
		individual diagnostics are also available.
		`)
	longDescriptionIndividual = templates.LongDesc(`
		Runs the %s diagnostic.

		%s
		`)
)

// NewCmdDiagnostics is the base command for running a standard set of diagnostics with generic options only.
func NewCmdDiagnostics(name string, fullName string, out io.Writer) *cobra.Command {
	available := availableDiagnostics()
	o := &DiagnosticsOptions{
		RequestedDiagnostics:     available.Names().Difference(defaultSkipDiagnostics()),
		ParameterizedDiagnostics: types.NewParameterizedDiagnosticMap(available...),
		LogOptions:               &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Diagnose common cluster problems",
		Long:  fmt.Sprintf(longDescription, fullName),
		Run:   util.CommandRunFunc(o),
	}
	cmd.SetOutput(out) // for output re: usage / help
	o.bindCommonFlags(cmd.Flags())
	o.bindClientFlags(cmd.Flags())
	o.bindHostFlags(cmd.Flags())

	// add "all" subcommand
	cmd.AddCommand(NewCmdDiagnosticsAll(AllDiagnosticsRecommendedName, fullName+" "+AllDiagnosticsRecommendedName, out, available))
	// add individual diagnostic subcommands
	for _, diag := range available {
		cmd.AddCommand(NewCmdDiagnosticsIndividual(strings.ToLower(diag.Name()), fullName+" "+strings.ToLower(diag.Name()), out, diag))
	}
	// add hidden in-pod subcommands
	cmd.AddCommand(
		poddiag.NewCommandPodDiagnostics(poddiag.InPodDiagnosticRecommendedName, out),
		networkpoddiag.NewCommandNetworkPodDiagnostics(networkpoddiag.InPodNetworkCheckRecommendedName, out),
	)

	return cmd
}

// NewCmdDiagnosticsAll is the command for running ALL diagnostics and providing all flags.
func NewCmdDiagnosticsAll(name string, fullName string, out io.Writer, available types.DiagnosticList) *cobra.Command {
	o := &DiagnosticsOptions{
		RequestedDiagnostics:     available.Names(),
		ParameterizedDiagnostics: types.NewParameterizedDiagnosticMap(available...),
		LogOptions:               &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Diagnose common cluster problems",
		Long:  fmt.Sprintf(longDescriptionAll, fullName),
		Run:   util.CommandRunFunc(o),
	}
	cmd.SetOutput(out) // for output re: usage / help
	o.bindCommonFlags(cmd.Flags())
	o.bindClientFlags(cmd.Flags())
	o.bindHostFlags(cmd.Flags())
	o.bindRequestedIndividualFlags(cmd.Flags())
	return cmd
}

// NewCmdDiagnosticsIndividual is a parameterized subcommand providing a single diagnostic and its flags.
func NewCmdDiagnosticsIndividual(name string, fullName string, out io.Writer, diagnostic types.Diagnostic) *cobra.Command {
	o := &DiagnosticsOptions{
		RequestedDiagnostics:     sets.NewString(diagnostic.Name()),
		ParameterizedDiagnostics: types.NewParameterizedDiagnosticMap(diagnostic),
		LogOptions:               &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:     name,
		Short:   diagnostic.Description(),
		Long:    fmt.Sprintf(longDescriptionIndividual, diagnostic.Name(), diagnostic.Description()),
		Run:     util.CommandRunFunc(o),
		Aliases: []string{diagnostic.Name()},
	}
	cmd.SetOutput(out) // for output re: usage / help
	o.bindCommonFlags(cmd.Flags())
	needClient, needHost := diagnostic.Requirements()
	if pd, ok := diagnostic.(types.ParameterizedDiagnostic); ok {
		bindIndividualFlags(pd, "", cmd.Flags())
	}
	if needClient {
		o.bindClientFlags(cmd.Flags())
	}
	if needHost {
		o.bindHostFlags(cmd.Flags())
	}
	return cmd
}

// returns the logger built according to options (must be Complete()ed)
func (o *DiagnosticsOptions) Logger() *log.Logger {
	return o.logger
}

// gather a list of all diagnostics that are available to be invoked by the main command
func availableDiagnostics() types.DiagnosticList {
	available := availableClientDiagnostics()
	available = append(available, availableClusterDiagnostics()...)
	available = append(available, availableHostDiagnostics()...)
	return available
}

// gather a list of diagnostic names to skip when running the main command
func defaultSkipDiagnostics() sets.String {
	toSkip := sets.NewString()
	toSkip.Insert(defaultSkipHostDiagnostics.List()...)
	return toSkip
}

// bind flags that are available on all user-facing commands
func (o *DiagnosticsOptions) bindCommonFlags(flags *flag.FlagSet) {
	flagtypes.GLog(flags)
	options.BindLoggerOptionFlags(flags, o.LogOptions, options.RecommendedLoggerOptionFlags())
}

// bind flags that are necessary for setting up an API client
func (o *DiagnosticsOptions) bindClientFlags(flags *flag.FlagSet) {
	o.ClientFlags = flag.NewFlagSet("client", flag.ContinueOnError) // hide the extensive set of client flags
	o.Factory = osclientcmd.New(o.ClientFlags)                      // that would otherwise be added to this command
	flags.AddFlag(o.ClientFlags.Lookup(config.OpenShiftConfigFlagName))
	flags.AddFlag(o.ClientFlags.Lookup("context")) // TODO: find k8s constant
	flags.StringVar(&o.ClientClusterContext, options.FlagClusterContextName, "", "Client context to use for cluster administrator")
	flags.BoolVar(&o.PreventModification, options.FlagPreventModificationName, false, "If true, may be set to prevent diagnostics making any changes via the API")
}

// bind flags that are used by host diagnostics
func (o *DiagnosticsOptions) bindHostFlags(flags *flag.FlagSet) {
	flags.StringVar(&o.MasterConfigLocation, options.FlagMasterConfigName, "", "Path to master config file (implies --host)")
	flags.StringVar(&o.NodeConfigLocation, options.FlagNodeConfigName, "", "Path to node config file (implies --host)")
	flags.BoolVar(&o.IsHost, options.FlagIsHostName, false, "If true, look for systemd and journald units even without master/node config")
}

// bind flags for all diagnostics that have their own parameters
func (o *DiagnosticsOptions) bindRequestedIndividualFlags(flags *flag.FlagSet) {
	for name, diag := range o.ParameterizedDiagnostics {
		if o.RequestedDiagnostics.Has(name) {
			bindIndividualFlags(diag, strings.ToLower(diag.Name()+"-"), flags)
		}
	}
}

// bind flags for parameters from a single diagnostic
func bindIndividualFlags(diag types.ParameterizedDiagnostic, prefix string, flags *flag.FlagSet) {
	for _, param := range diag.AvailableParameters() {
		name := prefix + param.Name
		switch target := param.Target.(type) {
		case *string:
			flags.StringVar(target, name, param.Default.(string), param.Description)
		case *int:
			flags.IntVar(target, name, param.Default.(int), param.Description)
		case *int64:
			flags.Int64Var(target, name, param.Default.(int64), param.Description)
		case *bool:
			flags.BoolVar(target, name, param.Default.(bool), param.Description)
		default:
			panic("Don't know what to do with parameter")
		}
	}
}

// Complete fills in DiagnosticsConfig needed if the command is actually invoked.
func (o *DiagnosticsOptions) Complete(c *cobra.Command, args []string) error {
	var err error
	o.logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	if len(args) > 0 {
		c.Usage()
		return fmt.Errorf("\nUnexpected command line argument(s): %v", args)
	}

	// If not given master/client config file locations, check if the defaults exist
	// and adjust the options accordingly:
	if len(o.MasterConfigLocation) == 0 {
		if _, err := os.Stat(StandardMasterConfigPath); !os.IsNotExist(err) {
			o.MasterConfigLocation = StandardMasterConfigPath
		}
	}
	if len(o.NodeConfigLocation) == 0 {
		if _, err := os.Stat(StandardNodeConfigPath); !os.IsNotExist(err) {
			o.NodeConfigLocation = StandardNodeConfigPath
		}
	}

	return nil
}

// RunDiagnostics builds diagnostics based on the options and executes them. Returns:
// error (raised during construction of diagnostics; may be an aggregate error object),
func (o DiagnosticsOptions) RunDiagnostics() error {
	diagnostics, failure := o.buildDiagnostics()
	if failure != nil {
		return failure
	}
	return util.RunDiagnostics(o.Logger(), diagnostics)
}

func (o DiagnosticsOptions) buildDiagnostics() (diags []types.Diagnostic, failure error) {
	diagnostics := []types.Diagnostic{}

	// don't trust discovery/build of diagnostics; wrap panic nicely in case of developer error
	defer func() {
		if r := recover(); r != nil {
			failure = fmt.Errorf("While building the diagnostics, a panic was encountered.\nThis is a bug in diagnostics. Error and stack trace follow: \n%v\n%s", r, debug.Stack())
		}
	}()

	// build client/cluster diags if there is a client config for them to use
	expected, detected := o.detectClientConfig() // may log and return problems
	if !expected {
		// no diagnostic required a client config, nothing to do
	} else if !detected {
		// there just plain isn't any client config file available
		o.Logger().Notice("CED3014", "No client configuration specified; skipping client and cluster diagnostics.")
	} else if rawConfig, err := o.buildRawConfig(); err != nil { // client config is totally broken - won't parse etc (problems may have been detected and logged)
		o.Logger().Error("CED3015", fmt.Sprintf("Client configuration failed to load; skipping client and cluster diagnostics due to error: %s", err.Error()))
	} else {
		clientDiags, err := o.buildClientDiagnostics(rawConfig)
		if err != nil {
			return diagnostics, err
		}
		diagnostics = append(diagnostics, clientDiags...)

		clusterDiags, err := o.buildClusterDiagnostics(rawConfig)
		if err != nil {
			return diagnostics, err
		}
		diagnostics = append(diagnostics, clusterDiags...)
	}

	// build host diagnostics if config is available
	hostDiags, err := o.buildHostDiagnostics()
	if err != nil {
		return diagnostics, err
	}
	diagnostics = append(diagnostics, hostDiags...)

	// complete any diagnostics that require it
	errors := []error{}
	for _, d := range diagnostics {
		if toComplete, ok := d.(types.IncompleteDiagnostic); ok {
			if err := toComplete.Complete(o.Logger()); err != nil {
				errors = append(errors, err)
			}
		}
	}
	if len(errors) > 0 {
		return diagnostics, kutilerrors.NewAggregate(errors)
	}
	return diagnostics, nil
}
