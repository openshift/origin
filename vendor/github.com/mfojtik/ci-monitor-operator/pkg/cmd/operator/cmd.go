package operator

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/mfojtik/ci-monitor-operator/pkg/operator"
	"github.com/mfojtik/ci-monitor-operator/pkg/version"
)

func NewOperator() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("ci-monitor-operator", version.Get(), operator.RunOperator).
		NewCommand()
	cmd.Use = "operator"
	cmd.Short = "Start the OpenShift CI monitor operator"

	return cmd
}
