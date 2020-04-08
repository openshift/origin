package cmd

import (
	"github.com/spf13/cobra"

	"github.com/openshift/library-go/pkg/controller/controllercmd"

	"github.com/openshift/origin/pkg/monitor/resourcewatch/operator"
	"github.com/openshift/origin/pkg/version"
)

func NewRunResourceWatchCommand() *cobra.Command {
	cmd := controllercmd.
		NewControllerCommandConfig("run-resourcewatch", version.Get(), operator.RunOperator).
		NewCommand()
	cmd.Use = "run-resourcewatch"
	cmd.Short = "Run watching resource changes"

	return cmd
}
