package network

import (
	"fmt"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	"github.com/openshift/origin/pkg/network"
)

const MakeGlobalProjectsNetworkCommandName = "make-projects-global"

var (
	makeGlobalProjectsNetworkLong = templates.LongDesc(`
		Make project network global

		Allows projects to access all pods in the cluster and vice versa when using the %[1]s network plugin.`)

	makeGlobalProjectsNetworkExample = templates.Examples(`
		# Allow project p1 to access all pods in the cluster and vice versa
		%[1]s <p1>

		# Allow all projects with label name=share to access all pods in the cluster and vice versa
		%[1]s --selector='name=share'`)
)

type MakeGlobalOptions struct {
	Options *ProjectOptions
}

func NewMakeGlobalOptions(streams genericclioptions.IOStreams) *MakeGlobalOptions {
	return &MakeGlobalOptions{
		Options: NewProjectOptions(streams),
	}
}

func NewCmdMakeGlobalProjectsNetwork(commandName, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewMakeGlobalOptions(streams)
	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Make project network global",
		Long:    fmt.Sprintf(makeGlobalProjectsNetworkLong, network.MultiTenantPluginName),
		Example: fmt.Sprintf(makeGlobalProjectsNetworkExample, fullName),
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
func (o *MakeGlobalOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
	if err := o.Options.Complete(f, c, args); err != nil {
		return err
	}
	o.Options.CheckSelector = c.Flag("selector").Changed
	return nil
}

func (o *MakeGlobalOptions) Validate() error {
	return o.Options.Validate()
}

func (o *MakeGlobalOptions) Run() error {
	projects, err := o.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		if err = o.Options.UpdatePodNetwork(project.Name, networkapihelpers.GlobalPodNetwork, ""); err != nil {
			errList = append(errList, fmt.Errorf("removing network isolation for project %q failed, error: %v", project.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
