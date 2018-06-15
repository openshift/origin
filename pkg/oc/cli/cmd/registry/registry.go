package registry

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

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
func NewCmd(fullName string, f *clientcmd.Factory, in io.Reader, out, errout io.Writer) *cobra.Command {
	image := &cobra.Command{
		Use:   "registry COMMAND",
		Short: "Commands for working with the registry",
		Long:  imageLong,
		Run:   cmdutil.DefaultSubCommandRun(errout),
	}

	name := fmt.Sprintf("%s registry", fullName)

	groups := ktemplates.CommandGroups{
		{
			Message: "Advanced commands:",
			Commands: []*cobra.Command{
				info.New(name, f, out, errout),
				login.New(name, f, out, errout),
			},
		},
	}
	groups.Add(image)
	templates.ActsAsRootCommand(image, []string{"options"}, groups...)
	return image
}
