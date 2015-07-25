package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/gonum/graph/encoding/dot"
	"github.com/spf13/cobra"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	StatusRecommendedName = "status"

	statusLong = `
Show a high level overview of the current project

This command will show services, deployment configs, build configurations, and active deployments.
If you have any misconfigured components information about them will be shown. For more information
about individual items, use the describe command (e.g. oc describe buildConfig,
oc describe deploymentConfig, oc describe service).

You can specify an output format of "-o dot" to have this command output the generated status
graph in DOT format that is suitable for use by the "dot" command.`

	statusExample = `  // Show an overview of the current project
  $ %[1]s`
)

// NewCmdStatus implements the OpenShift cli status command
func NewCmdStatus(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	outputFormat := ""

	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show an overview of the current project",
		Long:    statusLong,
		Example: fmt.Sprintf(statusExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if strings.ToLower(outputFormat) == "dot" {
				cmdutil.CheckErr(RunGraph(f, out))
				return
			}

			cmdutil.CheckErr(RunStatus(f, out))
		},
	}

	cmd.Flags().StringVarP(&outputFormat, "output", "o", outputFormat, "Output format. One of: dot.")

	return cmd
}

// RunStatus contains all the necessary functionality for the OpenShift cli status command
func RunStatus(f *clientcmd.Factory, out io.Writer) error {
	client, kclient, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	describer := &describe.ProjectStatusDescriber{kclient, client}
	s, err := describer.Describe(namespace, "")
	if err != nil {
		return err
	}

	fmt.Fprintf(out, s)
	return nil
}

// RunGraph contains all the necessary functionality for the OpenShift cli graph command
func RunGraph(f *clientcmd.Factory, out io.Writer) error {
	client, kclient, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, _, err := f.DefaultNamespace()
	if err != nil {
		return err
	}

	describer := &describe.ProjectStatusDescriber{kclient, client}
	g, _, err := describer.MakeGraph(namespace)
	if err != nil {
		return err
	}

	data, err := dot.Marshal(g, namespace, "", "  ", false)
	if err != nil {
		return err
	}

	fmt.Fprintf(out, "%s", string(data))
	return nil
}
