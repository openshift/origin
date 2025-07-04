package run_resource_watch

import (
	"fmt"

	"github.com/openshift/origin/pkg/resourcewatch/operator"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunResourceWatchCommand() *cobra.Command {
	var toJsonPath, fromJsonPath string

	cmd := &cobra.Command{
		Use:   "run-resourcewatch",
		Short: "Run watch for resource changes and commit each to a git repository",
		Long: templates.LongDesc(`
			Watches specific resources using the given kubeconfig for create/update/delete,
			and commits the latest state of the resource to a git repo. This allows you to
			see precisely how a resource changed over time.
			By default /repository will be used, specify REPOSITORY_PATH env var to
			override.
			Sample invocation against an external cluster:
			  $ REPOSITORY_PATH="/tmp/resource-watch-repo" openshift-tests run-resourcewatch --kubeconfig /path/to/kubeconfig --namespace default
		`),

		SilenceUsage:  true,
		SilenceErrors: true,
		PreRunE: func(cmd *cobra.Command, args []string) error {
			// Check for mutual exclusivity of --to-json and --from-json
			if toJsonPath != "" && fromJsonPath != "" {
				return fmt.Errorf("--to-json and --from-json are mutually exclusive")
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			return operator.RunResourceWatch(toJsonPath, fromJsonPath)
		},
	}
	var dummy string
	cmd.Flags().StringVar(&dummy, "kubeconfig", "", "This option is not used any more. It will be removed in later releases")
	cmd.Flags().StringVar(&dummy, "namespace", "", "This option is not used any more. It will be removed in later releases")
	cmd.Flags().StringVar(&toJsonPath, "to-json", "", "Path to JSON file for output (mutually exclusive with --from-json)")
	cmd.Flags().StringVar(&fromJsonPath, "from-json", "", "Path to JSON file for input (mutually exclusive with --to-json)")

	return cmd
}
