package importer

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/oc/cli/importer/appjson"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

var (
	importLong = templates.LongDesc(`
		Import outside applications into OpenShift

		These commands assist in bringing existing applications into OpenShift.`)
)

// NewCmdImport exposes commands for modifying objects.
func NewCmdImport(fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import COMMAND",
		Short: "Commands that import applications",
		Long:  importLong,
		Run:   kcmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s import", fullName)

	cmd.AddCommand(appjson.NewCmdAppJSON(name, f, streams))
	return cmd
}
