package healthcheckpkg

import (
	healthcheck "github.com/openshift/origin/pkg/health-check"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewTestFailureClusterHealthCheckCommand() *cobra.Command {
	healthCheckOpts := &healthcheck.Options{}

	cmd := &cobra.Command{
		Use:   "health-check",
		Short: "Performs health check against the cluster",
		Long: templates.LongDesc(`
Check cluster health, including:
all machines should be in Running state;
all nodes should be ready;
ready node count should match or exceed running machine count;
operators health ordered by dependency;
`),
		RunE: func(cmd *cobra.Command, args []string) error {
			return healthCheckOpts.Run()
		},
	}
	cmd.Flags().StringVar(&healthCheckOpts.JUnitDir,
		"junit-dir", healthCheckOpts.JUnitDir,
		"The directory where test reports were written, and operators health file will be stored.")

	return cmd
}
