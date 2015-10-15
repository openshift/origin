package network

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/openshift-sdn/pkg/ovssubnet"
	"github.com/openshift/openshift-sdn/plugins/osdn/multitenant"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	UnIsolateProjectsNetworkCommandName = "unisolate-projects"

	unIsolateProjectsNetworkLong = `
UnIsolate project network

Allows projects to access all pods in the cluster and vice versa when using the %[1]s network plugin.`

	unIsolateProjectsNetworkExample = `	# Allow project p1 to access all pods in the cluster and vice versa
	$ %[1]s <p1>

	# Allow all projects with label name=share to access all pods in the cluster and vice versa
	$ %[1]s --selector='name=share'`
)

type UnIsolateOptions struct {
	Options *ProjectOptions
}

func NewCmdUnIsolateProjectsNetwork(commandName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ProjectOptions{}
	unIsolateOp := &UnIsolateOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "UnIsolate project network",
		Long:    fmt.Sprintf(unIsolateProjectsNetworkLong, multitenant.NetworkPluginName()),
		Example: fmt.Sprintf(unIsolateProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			err := unIsolateOp.Run()
			kcmdutil.CheckErr(err)
		},
	}
	flags := cmd.Flags()

	// Common optional params
	flags.StringVar(&opts.Selector, "selector", "", "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (u *UnIsolateOptions) Run() error {
	projects, err := u.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		err = u.Options.CreateOrUpdateNetNamespace(project.ObjectMeta.Name, ovssubnet.AdminVNID)
		if err != nil {
			errList = append(errList, fmt.Errorf("Removing network isolation for project '%s' failed, error: %v", project.ObjectMeta.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
