package release

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

func NewCmd(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release",
		Short: "Tools for managing the OpenShift release process",
		Long: templates.LongDesc(`
			This tool is used by OpenShift release to build images that can update a cluster.

			The subcommands allow you to see information about releases, perform administrative
			actions inspect the content of the release, and mirror release content across image
			registries.
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
