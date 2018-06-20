package registry

import (
	"fmt"

	"github.com/spf13/cobra"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/oc/cli/cmd/registry/info"
	"github.com/openshift/origin/pkg/oc/cli/cmd/registry/login"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	imageLong = ktemplates.LongDesc(`
		Manage the integrated registry on OpenShift

		These commands help you work with an integrated OpenShift registry.`)
)

// NewCmd exposes commands for working with the registry.
func NewCmd(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	image := &cobra.Command{
		Use:   "registry COMMAND",
		Short: "Commands for working with the registry",
		Long:  imageLong,
		Run:   cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s registry", fullName)

	groups := ktemplates.CommandGroups{
		{
			Message: "Advanced commands:",
			Commands: []*cobra.Command{
				info.New(name, f, streams),
				login.New(name, f, streams),
			},
		},
	}
	groups.Add(image)
	templates.ActsAsRootCommand(image, []string{"options"}, groups...)
	return image
}
