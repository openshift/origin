package e2e_analysis

import (
	e2eanalysis "github.com/openshift/origin/pkg/e2eanalysis"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewTestFailureClusterAnalysisCheckCommand() *cobra.Command {
	e2eAnalysisOpts := &e2eanalysis.Options{}

	cmd := &cobra.Command{
		Use:   "e2e-analysis",
		Short: "Performs analysis against the cluster",
		Long: templates.LongDesc(`
Check cluster health and run some e2e analyisis, including:
all machines should be in Running state;
all nodes should be ready;
ready node count should match or exceed running machine count;
operators health ordered by dependency;
and so on.`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return e2eAnalysisOpts.Run()
		},
	}
	cmd.Flags().StringVar(&e2eAnalysisOpts.JUnitDir,
		"junit-dir", e2eAnalysisOpts.JUnitDir,
		"The directory where test reports were written in junit xml format.")
	cmd.Flags().BoolVar(&e2eAnalysisOpts.SkipReadinessChecks,
		"skip-readiness-checks", e2eAnalysisOpts.SkipReadinessChecks,
		"Skip all cluster readiness checks and mark them as skipped in JUnit output. "+
			"This flag should almost never be set directly — it is intended ONLY for CI lanes "+
			"where the cluster is intentionally degraded (e.g. a node is shut down for the "+
			"duration of the test) and health checks are expected to fail. "+
			"Can also be enabled by setting the SKIP_READINESS_CHECKS=true environment variable.")

	return cmd
}
