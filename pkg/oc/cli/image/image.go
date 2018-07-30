package image

import (
	"fmt"

	"github.com/spf13/cobra"

	ktemplates "k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/oc/cli/image/append"
	"github.com/openshift/origin/pkg/oc/cli/image/extract"
	"github.com/openshift/origin/pkg/oc/cli/image/mirror"
)

var (
	imageLong = ktemplates.LongDesc(`
		Manage images on OpenShift

		These commands help you manage images on OpenShift.`)
)

// NewCmdImage exposes commands for modifying images.
func NewCmdImage(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	image := &cobra.Command{
		Use:   "image COMMAND",
		Short: "Useful commands for managing images",
		Long:  imageLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s image", fullName)

	groups := ktemplates.CommandGroups{
		{
			Message: "Advanced commands:",
			Commands: []*cobra.Command{
				append.NewCmdAppendImage(name, streams),
				extract.New(name, streams),
				mirror.NewCmdMirrorImage(name, streams),
			},
		},
	}
	groups.Add(image)
	templates.ActsAsRootCommand(image, []string{"options"}, groups...)
	return image
}
