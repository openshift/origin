package render

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"

	test_report "github.com/openshift/origin/pkg/cmd/openshift-tests/render/test-report"
)

func NewRenderCommand(streams genericclioptions.IOStreams) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "render",
		SilenceErrors: true,
	}
	cmd.AddCommand(
		test_report.NewRenderTestReportCommand(streams),
	)
	return cmd
}
