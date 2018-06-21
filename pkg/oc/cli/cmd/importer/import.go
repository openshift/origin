package importer

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

var (
	importLong = templates.LongDesc(`
		Import outside applications into OpenShift

		These commands assist in bringing existing applications into OpenShift.`)
)

// NewCmdImport exposes commands for modifying objects.
func NewCmdImport(fullName string, f *clientcmd.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "import COMMAND",
		Short: "Commands that import applications",
		Long:  importLong,
		Run:   cmdutil.DefaultSubCommandRun(streams.ErrOut),
	}

	name := fmt.Sprintf("%s import", fullName)

	cmd.AddCommand(NewCmdAppJSON(name, f, streams))
	return cmd
}
