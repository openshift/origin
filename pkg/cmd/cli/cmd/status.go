package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const statusLongDesc = `
Show a high level overview of the current project. Links components by their relationships.
For more information about individual items, use the describe command (e.g. osc describe buildConfig,
osc describe deploymentConfig, osc describe service).
`

func NewCmdStatus(fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "status",
		Short: "Show an overview of the current project",
		Long:  fmt.Sprintf(statusLongDesc, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			err := RunStatus(f, out)
			cmdutil.CheckErr(err)
		},
	}
	return cmd
}

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
