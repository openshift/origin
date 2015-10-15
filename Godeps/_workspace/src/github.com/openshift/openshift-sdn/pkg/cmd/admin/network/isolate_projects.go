package network

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/openshift-sdn/plugins/osdn/multitenant"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	IsolateProjectsNetworkCommandName = "isolate-projects"

	isolateProjectsNetworkLong = `
Isolate project network

Allows projects to isolate their network from other projects when using the %[1]s network plugin.`

	isolateProjectsNetworkExample = `	# Provide isolation for project p1
	$ %[1]s <p1>

	# Allow all projects with label name=top-secret to have their own isolated project network
	$ %[1]s --selector='name=top-secret'`
)

type IsolateOptions struct {
	Options *ProjectOptions
}

func NewCmdIsolateProjectsNetwork(commandName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ProjectOptions{}
	isolateOp := &IsolateOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Isolate project network",
		Long:    fmt.Sprintf(isolateProjectsNetworkLong, multitenant.NetworkPluginName()),
		Example: fmt.Sprintf(isolateProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			err := isolateOp.Run()
			kcmdutil.CheckErr(err)
		},
	}
	flags := cmd.Flags()

	// Common optional params
	flags.StringVar(&opts.Selector, "selector", "", "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (i *IsolateOptions) Run() error {
	projects, err := i.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		// TBD: Create or Update network namespace
		// TODO: Fix this once we move VNID allocation to REST layer
		errList = append(errList, fmt.Errorf("Project '%s' can not be isolated. Isolate project network feature yet to be implemented!", project.ObjectMeta.Name))
	}
	return kerrors.NewAggregate(errList)
}
