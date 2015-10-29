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
	MakeGlobalProjectsNetworkCommandName = "make-projects-global"

	makeGlobalProjectsNetworkLong = `
Make project network global

Allows projects to access all pods in the cluster and vice versa when using the %[1]s network plugin.`

	makeGlobalProjectsNetworkExample = `	# Allow project p1 to access all pods in the cluster and vice versa
	$ %[1]s <p1>

	# Allow all projects with label name=share to access all pods in the cluster and vice versa
	$ %[1]s --selector='name=share'`
)

type MakeGlobalOptions struct {
	Options *ProjectOptions
}

func NewCmdMakeGlobalProjectsNetwork(commandName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ProjectOptions{}
	makeGlobalOp := &MakeGlobalOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Make project network global",
		Long:    fmt.Sprintf(makeGlobalProjectsNetworkLong, multitenant.NetworkPluginName()),
		Example: fmt.Sprintf(makeGlobalProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			err := makeGlobalOp.Run()
			kcmdutil.CheckErr(err)
		},
	}
	flags := cmd.Flags()

	// Common optional params
	flags.StringVar(&opts.Selector, "selector", "", "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (m *MakeGlobalOptions) Run() error {
	projects, err := m.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		err = m.Options.CreateOrUpdateNetNamespace(project.ObjectMeta.Name, ovssubnet.AdminVNID)
		if err != nil {
			errList = append(errList, fmt.Errorf("Removing network isolation for project '%s' failed, error: %v", project.ObjectMeta.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
