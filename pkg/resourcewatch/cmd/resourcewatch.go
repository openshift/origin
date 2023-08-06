package cmd

import (
	"github.com/openshift/origin/pkg/resourcewatch/operator"
	"github.com/spf13/cobra"
	"k8s.io/kubectl/pkg/util/templates"
)

func NewRunResourceWatchCommand() *cobra.Command {
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
		RunE: func(cmd *cobra.Command, args []string) error {
			return operator.RunResourceWatch()
		},
	}
	var dummy string
	cmd.Flags().StringVar(&dummy, "kubeconfig", "", "This option is not used any more. It will be removed in later releases")
	cmd.Flags().StringVar(&dummy, "namespace", "", "This option is not used any more. It will be removed in later releases")
	return cmd
}
