package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	statusLong = `Show a high level overview of the current project. Links components by their relationships.
For more information about individual items, use the describe command (e.g. oc describe buildConfig,
oc describe deploymentConfig, oc describe service).`

	statusExample = `  // Show an overview of the current project
  $ %[1]s status`
)

// NewCmdStatus implements the OpenShift cli status command
func NewCmdStatus(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:     "status",
		Short:   "Show an overview of the current project",
		Long:    statusLong,
		Example: fmt.Sprintf(statusExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStatus(f, out)
			cmdutil.CheckErr(err)
		},
	}
	return cmd
}

// RunStatus contains all the necessary functionality for the OpenShift cli status command
func RunStatus(f *clientcmd.Factory, out io.Writer) error {
	client, kclient, err := f.Clients()
	if err != nil {
		return err
	}

	namespace, err := f.DefaultNamespace()
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
