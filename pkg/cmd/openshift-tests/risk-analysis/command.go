package risk_analysis

import (
	"os"

	"github.com/openshift/origin/pkg/riskanalysis"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

const sippyDefaultURL = "https://sippy.dptools.openshift.org/api/jobs/runs/risk_analysis"

func NewTestFailureRiskAnalysisCommand() *cobra.Command {
	riskAnalysisOpts := &riskanalysis.Options{
		Out:    os.Stdout,
		ErrOut: os.Stderr,
	}

	cmd := &cobra.Command{
		Use:   "risk-analysis",
		Short: "Performs risk analysis on test failures",
		Long: templates.LongDesc(`
Uses the test failure summary json files written along-side our junit xml
files after an invocation of openshift-tests. If multiple files are present
(multiple invocations of openshift-tests) we will merge them into one.
Results are then submitted to sippy which will return an analysis of per-test
and overall risk level given historical pass rates on the failed tests.
The resulting analysis is then also written to the junit artifacts directory.
`),

		RunE: func(cmd *cobra.Command, args []string) error {
			return riskAnalysisOpts.Run()
		},
	}
	cmd.Flags().StringVar(&riskAnalysisOpts.JUnitDir,
		"junit-dir", riskAnalysisOpts.JUnitDir,
		"The directory where test reports were written, and analysis file will be stored.")
	cmd.MarkFlagRequired("junit-dir")
	cmd.Flags().StringVar(&riskAnalysisOpts.SippyURL,
		"sippy-url", sippyDefaultURL,
		"Sippy URL API endpoint")
	return cmd
}
