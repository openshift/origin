package diagnostics

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	kutilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	diagnosticflags "github.com/openshift/origin/pkg/cmd/experimental/diagnostics/options"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/diagnostics/log"
)

var (
	AvailableOverallDiagnostics = util.NewStringSet()
)

func init() {
	AvailableOverallDiagnostics.Insert(AvailableClientDiagnostics.List()...)
	AvailableOverallDiagnostics.Insert(AvailableMasterDiagnostics.List()...)
	AvailableOverallDiagnostics.Insert(AvailableNodeDiagnostics.List()...)
}

type OverallDiagnosticsOptions struct {
	RequestedDiagnostics util.StringList

	MasterConfigLocation string
	NodeConfigLocation   string

	Factory *osclientcmd.Factory

	LogOptions *log.LoggerOptions
	Logger     *log.Logger
}

const longAllDescription = `
OpenShift Diagnostics

This command helps you understand and troubleshoot OpenShift. It is
intended to be run from the same context as an OpenShift client or running
master / node in order to troubleshoot from the perspective of each.

    $ %[1]s

If run without flags or subcommands, it will check for config files for
client, master, and node, and if found, use them for troubleshooting
those components. If master/node config files are not found, the tool
assumes they are not present and does diagnostics only as a client.

You may also specify config files explicitly with flags below, in which
case you will receive an error if they are invalid or not found.

    $ %[1]s --master-config=/etc/openshift/master/master-config.yaml

Subcommands may be used to scope the troubleshooting to a particular
component and are not limited to using config files; you can and should
use the same flags that are actually set on the command line for that
component to configure the diagnostic.

    $ %[1]s node --hostname='node.example.com' --kubeconfig=...

NOTE: This is an alpha version of diagnostics and will change significantly.
NOTE: Global flags (from the 'options' subcommand) are ignored here but
can be used with subcommands.
`

func NewCommandDiagnostics(name string, fullName string, out io.Writer) *cobra.Command {
	o := &OverallDiagnosticsOptions{
		RequestedDiagnostics: AvailableOverallDiagnostics.List(),
		LogOptions:           &log.LoggerOptions{Out: out},
	}

	cmd := &cobra.Command{
		Use:   name,
		Short: "This utility helps you understand and troubleshoot OpenShift v3.",
		Long:  fmt.Sprintf(longAllDescription, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete())

			failed, err := o.RunDiagnostics()
			o.Logger.Summary()
			o.Logger.Finish()

			kcmdutil.CheckErr(err)
			if failed {
				os.Exit(255)
			}

		},
	}
	cmd.SetOutput(out) // for output re: usage / help

	o.Factory = osclientcmd.New(cmd.Flags()) // side effect: add standard persistent flags for openshift client
	cmd.Flags().StringVar(&o.MasterConfigLocation, "master-config", "", "path to master config file")
	cmd.Flags().StringVar(&o.NodeConfigLocation, "node-config", "", "path to node config file")
	diagnosticflags.BindLoggerOptionFlags(cmd.Flags(), o.LogOptions, diagnosticflags.RecommendedLoggerOptionFlags())
	diagnosticflags.BindDiagnosticFlag(cmd.Flags(), &o.RequestedDiagnostics, diagnosticflags.NewRecommendedDiagnosticFlag())

	cmd.AddCommand(NewClientCommand(ClientDiagnosticsRecommendedName, name+" "+ClientDiagnosticsRecommendedName, out))
	cmd.AddCommand(NewMasterCommand(MasterDiagnosticsRecommendedName, name+" "+MasterDiagnosticsRecommendedName, out))
	cmd.AddCommand(NewNodeCommand(NodeDiagnosticsRecommendedName, name+" "+NodeDiagnosticsRecommendedName, out))
	cmd.AddCommand(NewOptionsCommand())

	return cmd
}

func (o *OverallDiagnosticsOptions) Complete() error {
	var err error
	o.Logger, err = o.LogOptions.NewLogger()
	if err != nil {
		return err
	}

	return nil
}

func (o OverallDiagnosticsOptions) RunDiagnostics() (bool, error) {
	failed := false
	errors := []error{}

	masterFailed, err := o.CheckMaster()
	failed = failed && masterFailed
	if err != nil {
		errors = append(errors, err)
	}

	nodeFailed, err := o.CheckNode()
	failed = failed && nodeFailed
	if err != nil {
		errors = append(errors, err)
	}

	clientFailed, err := o.CheckClient()
	failed = failed && clientFailed
	if err != nil {
		errors = append(errors, err)
	}

	return failed, kutilerrors.NewAggregate(errors)
}

func (o OverallDiagnosticsOptions) CheckClient() (bool, error) {
	runClientChecks := true

	_, kubeClient, err := o.Factory.Clients()
	if err != nil {
		runClientChecks = false
	}

	kubeConfig, err := o.Factory.OpenShiftClientConfig.RawConfig()
	if err != nil {
		runClientChecks = false
	}

	if runClientChecks {
		clientDiagnosticOptions := &ClientDiagnosticsOptions{
			RequestedDiagnostics: intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableClientDiagnostics).List(),
			KubeClient:           kubeClient,
			KubeConfig:           &kubeConfig,
			LogOptions:           o.LogOptions,
			Logger:               o.Logger,
		}

		return clientDiagnosticOptions.RunDiagnostics()
	}

	return false, nil
}

func (o OverallDiagnosticsOptions) CheckNode() (bool, error) {
	if len(o.NodeConfigLocation) == 0 {
		if _, err := os.Stat(StandardNodeConfigPath); !os.IsNotExist(err) {
			o.NodeConfigLocation = StandardNodeConfigPath
		}
	}

	if len(o.NodeConfigLocation) != 0 {
		masterDiagnosticOptions := &NodeDiagnosticsOptions{
			RequestedDiagnostics: intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableNodeDiagnostics).List(),
			NodeConfigLocation:   o.NodeConfigLocation,
			LogOptions:           o.LogOptions,
			Logger:               o.Logger,
		}

		return masterDiagnosticOptions.RunDiagnostics()
	}

	return false, nil
}

func (o OverallDiagnosticsOptions) CheckMaster() (bool, error) {
	if len(o.MasterConfigLocation) == 0 {
		if _, err := os.Stat(StandardMasterConfigPath); !os.IsNotExist(err) {
			o.MasterConfigLocation = StandardMasterConfigPath
		}
	}

	if len(o.MasterConfigLocation) != 0 {
		masterDiagnosticOptions := &MasterDiagnosticsOptions{
			RequestedDiagnostics: intersection(util.NewStringSet(o.RequestedDiagnostics...), AvailableMasterDiagnostics).List(),
			MasterConfigLocation: o.MasterConfigLocation,
			LogOptions:           o.LogOptions,
			Logger:               o.Logger,
		}

		return masterDiagnosticOptions.RunDiagnostics()
	}

	return false, nil
}

func NewOptionsCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use: "options",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Usage()
		},
	}

	templates.UseOptionsTemplates(cmd)

	return cmd
}

// TODO move upstream
func intersection(s1 util.StringSet, s2 util.StringSet) util.StringSet {
	result := util.NewStringSet()
	for key := range s1 {
		if s2.Has(key) {
			result.Insert(key)
		}
	}
	return result
}
