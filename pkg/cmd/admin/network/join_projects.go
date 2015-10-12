package network

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	kerrors "k8s.io/kubernetes/pkg/util/errors"

	"github.com/openshift/openshift-sdn/plugins/osdn/multitenant"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	JoinProjectsNetworkCommandName = "join-projects"

	joinProjectsNetworkLong = `
Join project network

Allows projects to join existing project network when using the %[1]s network plugin.`

	joinProjectsNetworkExample = `	# Allow project p2 to use project p1 network
	$ %[1]s --to=<p1> <p2>

	# Allow all projects with label name=top-secret to use project p1 network
	$ %[1]s --to=<p1> --selector='name=top-secret'`
)

type JoinOptions struct {
	Options *ProjectOptions

	joinProjectName string
}

func NewCmdJoinProjectsNetwork(commandName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ProjectOptions{}
	joinOp := &JoinOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Join project network",
		Long:    fmt.Sprintf(joinProjectsNetworkLong, multitenant.NetworkPluginName()),
		Example: fmt.Sprintf(joinProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := joinOp.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(c, err.Error()))
			}

			err := joinOp.Run()
			kcmdutil.CheckErr(err)
		},
	}
	flags := cmd.Flags()

	// Supported operations
	flags.StringVar(&joinOp.joinProjectName, "to", "", "Join network of the given project name")

	// Common optional params
	flags.StringVar(&opts.Selector, "selector", "", "Label selector to filter projects. Either pass one/more projects as arguments or use this project selector")

	return cmd
}

func (j *JoinOptions) Validate() error {
	errList := []error{}
	if err := j.Options.Validate(); err != nil {
		errList = append(errList, err)
	}
	if len(j.joinProjectName) == 0 {
		errList = append(errList, errors.New("must provide --to=<project_name>"))
	}
	return kerrors.NewAggregate(errList)
}

func (j *JoinOptions) Run() error {
	netID, err := j.Options.GetNetID(j.joinProjectName)
	if err != nil {
		return err
	}
	projects, err := j.Options.GetProjects()
	if err != nil {
		return err
	}

	errList := []error{}
	for _, project := range projects {
		err = j.Options.CreateOrUpdateNetNamespace(project.ObjectMeta.Name, netID)
		if err != nil {
			errList = append(errList, fmt.Errorf("Project '%s' failed to join '%s', error: %v", project.ObjectMeta.Name, j.joinProjectName, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
