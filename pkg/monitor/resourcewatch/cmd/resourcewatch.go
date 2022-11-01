package cmd

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/origin/pkg/monitor/resourcewatch/operator"
	"github.com/openshift/origin/pkg/version"
)

func NewRunResourceWatchCommand() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("run-resourcewatch", version.Get(), operator.RunOperator).
		NewCommandWithContext(context.TODO())
	cmd.Use = "run-resourcewatch"
	cmd.Short = "Run watch for resource changes and commit each to a git repository"
	cmd.Long = `
Watches specific resources using the given kubeconfig for create/update/delete,
and commits the latest state of the resource to a git repo. This allows you to
see precisely how a resource changed over time.
By default /repository will be used, specify REPOSITORY_PATH env var to
override.
Sample invocation against an external cluster:
  $ REPOSITORY_PATH="/tmp/resource-watch-repo" openshift-tests run-resourcewatch --kubeconfig /path/to/kubeconfig --namespace default
`

	return cmd
}
