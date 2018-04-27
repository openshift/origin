package network

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/network"
	networkapi "github.com/openshift/origin/pkg/network/apis/network"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
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

func NewCmdMakeGlobalProjectsNetwork(commandName, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	opts := &ProjectOptions{}
	makeGlobalOp := &MakeGlobalOptions{Options: opts}

	cmd := &cobra.Command{
		Use:     commandName,
		Short:   "Make project network global",
		Long:    fmt.Sprintf(makeGlobalProjectsNetworkLong, network.MultiTenantPluginName),
		Example: fmt.Sprintf(makeGlobalProjectsNetworkExample, fullName),
		Run: func(c *cobra.Command, args []string) {
			if err := opts.Complete(f, c, args, out); err != nil {
				kcmdutil.CheckErr(err)
			}
			opts.CheckSelector = c.Flag("selector").Changed
			if err := opts.Validate(); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageErrorf(c, err.Error()))
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
		if err = m.Options.UpdatePodNetwork(project.Name, networkapi.GlobalPodNetwork, ""); err != nil {
			errList = append(errList, fmt.Errorf("removing network isolation for project %q failed, error: %v", project.Name, err))
		}
	}
	return kerrors.NewAggregate(errList)
}
