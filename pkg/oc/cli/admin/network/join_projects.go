package network

import (
	"errors"
	"fmt"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/library-go/pkg/network/networkapihelpers"
	"github.com/openshift/origin/pkg/network"
)

const JoinProjectsNetworkCommandName = "join-projects"

var (
	joinProjectsNetworkLong = templates.LongDesc(`
		Join project network

		Allows projects to join existing project network when using the %[1]s network plugin.`)

	joinProjectsNetworkExample = templates.Examples(`
		# Allow project p2 to use project p1 network
		%[1]s --to=<p1> <p2>

		# Allow all projects with label name=top-secret to use project p1 network
		%[1]s --to=<p1> --selector='name=top-secret'`)
)

type JoinOptions struct {
	Options *ProjectOptions

	joinProjectName string
}

func NewJoinOptions(streams genericclioptions.IOStreams) *JoinOptions {
	return &JoinOptions{
		Options: NewProjectOptions(streams),
	}
}

func NewCmdJoinProjectsNetwork(commandName, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewJoinOptions(streams)
	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Join project network",
		Long:    fmt.Sprintf(joinProjectsNetworkLong, network.MultiTenantPluginName),
		Example: fmt.Sprintf(joinProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f, c, args))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}
	// Supported operations
	cmd.Flags().StringVar(&o.joinProjectName, "to", o.joinProjectName, "Join network of the given project name")

	// Common optional params
	cmd.Flags().StringVar(&o.Options.Selector, "selector", o.Options.Selector, "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (o *JoinOptions) Complete(f kcmdutil.Factory, c *cobra.Command, args []string) error {
	if err := o.Options.Complete(f, c, args); err != nil {
		return err
	}
	o.Options.CheckSelector = c.Flag("selector").Changed
	return nil
}

func (o *JoinOptions) Validate() error {
	errList := []error{}
	if err := o.Options.Validate(); err != nil {
		errList = append(errList, err)
	}
	if len(o.joinProjectName) == 0 {
		errList = append(errList, errors.New("must provide --to=<project_name>"))
	}
	return kerrors.NewAggregate(errList)
}

func (o *JoinOptions) Run() error {
	projects, err := o.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		if project.Name != o.joinProjectName {
			if err = o.Options.UpdatePodNetwork(project.Name, networkapihelpers.JoinPodNetwork, o.joinProjectName); err != nil {
				errList = append(errList, fmt.Errorf("project %q failed to join %q, error: %v", project.Name, o.joinProjectName, err))
			}
		}
	}
	return kerrors.NewAggregate(errList)
}
