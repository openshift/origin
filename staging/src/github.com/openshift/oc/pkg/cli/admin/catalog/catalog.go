package catalog

import (
	"github.com/spf13/cobra"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/util/templates"
)

func NewCmd(f kcmdutil.Factory, parentName string, streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "catalog",
		Short: "Tools for managing the OpenShift OLM Catalogs",
		Long: templates.LongDesc(`
			This tool is used to extract and mirror the contents of catalogs for Operator
			Lifecycle Manager.

			The subcommands allow you to build catalog images from a source (such as appregistry) 
			and mirror its content across registries.
			`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return cmd.Help()
		},
	}
	cmd.AddCommand(NewBuildImage(f, parentName+" catalog", streams))
	return cmd
}
