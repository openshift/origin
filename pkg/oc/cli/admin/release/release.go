package release

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
)

func NewCmd(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Tools for managing the OpenShift release process",
		Long: templates.LongDesc(`
			This tool is used by OpenShift release to build upgrade payloads.

			Experimental: This command is under active development and may change without notice.
			`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewInfo(f, parentName+" release", streams))
	cmd.AddCommand(NewRelease(f, parentName+" release", streams))
	cmd.AddCommand(NewExtract(f, parentName+" release", streams))
	cmd.AddCommand(NewMirror(f, parentName+" release", streams))
	return cmd
}
