package generate

import (
	"github.com/openshift/origin/pkg/cmd/openshift-tests/generate/durations"
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
)

func NewGenerateCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "generate",
		Short:         "Generate test data and artifacts",
		Long:          "Commands for generating test-related data and artifacts from various sources",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		durations.NewDurationsCommand(streams),
	)
	return cmd
}
