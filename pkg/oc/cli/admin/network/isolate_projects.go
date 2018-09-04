package network

import (
	"fmt"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	"github.com/openshift/origin/pkg/network"
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

func NewIsolateOptions(streams genericclioptions.IOStreams) *IsolateOptions {
	return &IsolateOptions{
		Options: NewProjectOptions(streams),
	}
}

func NewCmdIsolateProjectsNetwork(commandName, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewIsolateOptions(streams)
	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Isolate project network",
		Long:    fmt.Sprintf(isolateProjectsNetworkLong, network.MultiTenantPluginName),
		Example: fmt.Sprintf(isolateProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, c, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	// Common optional params
	cmd.Flags().StringVar(&o.Options.Selector, "selector", o.Options.Selector, "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (o *IsolateOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
	if err := o.Options.Complete(f, c, args); err != nil {
		return err
	}
	o.Options.CheckSelector = c.Flag("selector").Changed
	return nil
}

func (o *IsolateOptions) Validate() error {
	return o.Options.Validate()
}

func (o *IsolateOptions) Run() error {
	projects, err := o.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		if project.Name == kapi.NamespaceDefault {
			errList = append(errList, fmt.Errorf("network isolation for project %q is forbidden", project.Name))
			continue
		}
		if err = o.Options.UpdatePodNetwork(project.Name, networkapihelpers.IsolatePodNetwork, ""); err != nil {
			errList = append(errList, fmt.Errorf("network isolation for project %q failed, error: %v", project.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
