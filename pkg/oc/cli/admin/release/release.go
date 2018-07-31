package release

import (
	"github.com/spf13/cobra"

	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"
)

func NewCmd(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Tools for managing the OpenShift release process",
		Long: templates.LongDesc(`
			This tool is used by OpenShift release to build upgrade payloads.

			Experimental: This command is under active development and may change without notice.
			`),
	}
	cmd.AddCommand(NewRelease(f, streams))
	cmd.AddCommand(NewExtract(f, streams))
	return cmd
}
