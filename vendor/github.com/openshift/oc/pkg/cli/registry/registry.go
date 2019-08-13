package registry

import (
	"fmt"

	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/util/templates"

	"github.com/openshift/oc/pkg/cli/registry/info"
	"github.com/openshift/oc/pkg/cli/registry/login"
	cmdutil "github.com/openshift/oc/pkg/helpers/cmd"
)

var (
	imageLong = ktemplates.LongDesc(`
		Manage the integrated registry on OpenShift

		These commands help you work with an integrated OpenShift registry.`)
)

// NewCmd exposes commands for working with the registry.
func NewCmd(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	image := &cobra.Command{
		Use:   "registry COMMAND",
		Short: "Commands for working with the registry",
		Long:  imageLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s registry", fullName)

	groups := ktemplates.CommandGroups{
		{
			Message: "Advanced commands:",
			Commands: []*cobra.Command{
				info.NewRegistryInfoCmd(name, f, streams),
				login.NewRegistryLoginCmd(name, f, streams),
			},
		},
	}
	groups.Add(image)
	cmdutil.ActsAsRootCommand(image, []string{"options"}, groups...)
	return image
}
