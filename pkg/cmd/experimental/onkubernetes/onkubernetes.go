package onkubernetes

import (
	"io"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/admin"
	"github.com/openshift/origin/pkg/cmd/experimental/onkubernetes/install"
	"github.com/openshift/origin/pkg/cmd/experimental/onkubernetes/remove"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	onKubernetesLong = `The commands grouped here allow you to run OpenShift on an exiting Kubernetes cluster.`
)

// OnKubernetesRecommendedCommandName is the recommended command name
const OnKubernetesRecommendedCommandName = "on-kubernetes"

// NewCmdOnKubernetes groups commands that let you run OpenShift on an existing Kubernetes cluster.
func NewCmdOnKubernetes(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   name,
		Short: "Manage OpenShift on Kubernetes.",
		Long:  "The commands grouped here allow you to run OpenShift on an exiting Kubernetes cluster.",
		BashCompletionFunction: admin.BashCompletionFunc,
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(out)
			c.Help()
		},
	}

	cmd.AddCommand(install.NewCmdInstall(install.InstallRecommendedCommandName, fullName+" "+install.InstallRecommendedCommandName, f, out))
	cmd.AddCommand(remove.NewCmdRemove(remove.RemoveRecommendedCommandName, fullName+" "+remove.RemoveRecommendedCommandName, f, out))
	return cmd
}
