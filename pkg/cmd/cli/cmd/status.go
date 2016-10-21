package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/gonum/graph/encoding/dot"
	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	dotutil "github.com/openshift/origin/pkg/util/dot"
)

// StatusRecommendedName is the recommended command name.
const StatusRecommendedName = "status"

var (
	statusLong = templates.LongDesc(`
		Show a high level overview of the current project

		This command will show services, deployment configs, build configurations, and active deployments.
		If you have any misconfigured components information about them will be shown. For more information
		about individual items, use the describe command (e.g. %[1]s describe buildConfig,
		%[1]s describe deploymentConfig, %[1]s describe service).

		You can specify an output format of "-o dot" to have this command output the generated status
		graph in DOT format that is suitable for use by the "dot" command.`)

	statusExample = templates.Examples(`
		# See an overview of the current project.
	  %[1]s

	  # Export the overview of the current project in an svg file.
	  %[1]s -o dot | dot -T svg -o project.svg

	  # See an overview of the current project including details for any identified issues.
	  %[1]s -v`)
)

// StatusOptions contains all the necessary options for the Openshift cli status command.
type StatusOptions struct {
	namespace     string
	allNamespaces bool
	outputFormat  string
	describer     *describe.ProjectStatusDescriber
	out           io.Writer
	verbose       bool

	logsCommandName             string
	securityPolicyCommandFormat string
	setProbeCommandName         string
	patchCommandName            string
}

// NewCmdStatus implements the OpenShift cli status command.
// baseCLIName is the path from root cmd to the parent of this cmd.
func NewCmdStatus(name, baseCLIName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &StatusOptions{}

	cmd := &cobra.Command{
		Use:     fmt.Sprintf("%s [-o dot | -v ]", StatusRecommendedName),
		Short:   "Show an overview of the current project",
		Long:    fmt.Sprintf(statusLong, baseCLIName),
		Example: fmt.Sprintf(statusExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := opts.Complete(f, cmd, baseCLIName, args, out)
			kcmdutil.CheckErr(err)

			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			err = opts.RunStatus()
			kcmdutil.CheckErr(err)
		},
	}

	cmd.Flags().StringVarP(&opts.outputFormat, "output", "o", opts.outputFormat, "Output format. One of: dot.")
	cmd.Flags().BoolVarP(&opts.verbose, "verbose", "v", opts.verbose, "See details for resolving issues.")
	cmd.Flags().BoolVar(&opts.allNamespaces, "all-namespaces", false, "Display status for all namespaces (must have cluster admin)")

	return cmd
}

// Complete completes the options for the Openshift cli status command.
func (o *StatusOptions) Complete(f *clientcmd.Factory, cmd *cobra.Command, baseCLIName string, args []string, out io.Writer) error {
	if len(args) > 0 {
		return kcmdutil.UsageError(cmd, "no arguments should be provided")
	}

	o.logsCommandName = fmt.Sprintf("%s logs", cmd.Parent().CommandPath())
	o.securityPolicyCommandFormat = "oadm policy add-scc-to-user anyuid -n %s -z %s"
	o.setProbeCommandName = fmt.Sprintf("%s set probe", cmd.Parent().CommandPath())

	client, kclient, err := f.Clients()
	if err != nil {
		return err
	}

	config, err := f.OpenShiftClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	if o.allNamespaces {
		o.namespace = kapi.NamespaceAll
	} else {
		namespace, _, err := f.DefaultNamespace()
		if err != nil {
			return err
		}
		o.namespace = namespace
	}

	if baseCLIName == "" {
		baseCLIName = "oc"
	}

	o.describer = &describe.ProjectStatusDescriber{
		K:       kclient,
		C:       client,
		Server:  config.Host,
		Suggest: o.verbose,

		CommandBaseName: baseCLIName,

		// TODO: Remove these and reference them inside the markers using constants.
		LogsCommandName:             o.logsCommandName,
		SecurityPolicyCommandFormat: o.securityPolicyCommandFormat,
		SetProbeCommandName:         o.setProbeCommandName,
	}

	o.out = out

	return nil
}

// Validate validates the options for the Openshift cli status command.
func (o StatusOptions) Validate() error {
	if len(o.outputFormat) != 0 && o.outputFormat != "dot" {
		return fmt.Errorf("invalid output format provided: %s", o.outputFormat)
	}
	if len(o.outputFormat) > 0 && o.verbose {
		return errors.New("cannot provide suggestions when output format is dot")
	}
	return nil
}

// RunStatus contains all the necessary functionality for the OpenShift cli status command.
func (o StatusOptions) RunStatus() error {
	var (
		s   string
		err error
	)

	switch o.outputFormat {
	case "":
		s, err = o.describer.Describe(o.namespace, "")
		if err != nil {
			return err
		}
	case "dot":
		g, _, err := o.describer.MakeGraph(o.namespace)
		if err != nil {
			return err
		}
		data, err := dot.Marshal(g, dotutil.Quote(o.namespace), "", "  ", false)
		if err != nil {
			return err
		}
		s = string(data)
	default:
		return fmt.Errorf("invalid output format provided: %s", o.outputFormat)
	}

	fmt.Fprintf(o.out, s)
	return nil
}
