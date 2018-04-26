package network

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

const IsolateProjectsNetworkCommandName = "isolate-projects"

var (
	isolateProjectsNetworkLong = templates.LongDesc(`
		Isolate project network

		Allows projects to isolate their network from other projects when using the %[1]s network plugin.`)

	isolateProjectsNetworkExample = templates.Examples(`
		# Provide isolation for project p1
		%[1]s <p1>

		# Allow all projects with label name=top-secret to have their own isolated project network
		%[1]s --selector='name=top-secret'`)
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
		Long:    fmt.Sprintf(isolateProjectsNetworkLong, network.MultiTenantPluginName),
		Example: fmt.Sprintf(isolateProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
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
		if project.Name == kapi.NamespaceDefault {
			errList = append(errList, fmt.Errorf("network isolation for project %q is forbidden", project.Name))
			continue
		}
		if err = i.Options.UpdatePodNetwork(project.Name, networkapi.IsolatePodNetwork, ""); err != nil {
			errList = append(errList, fmt.Errorf("network isolation for project %q failed, error: %v", project.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
