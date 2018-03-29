package openshift_experimental

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/apiserver-operator"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/controller-operator"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/orchestration-operator"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator"
)

func NewExperimentalCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experimental",
		Short: "Experimental commands for OpenShift",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(apiserver_operator.NewAPIServerOperatorCommand(apiserver_operator.RecommendedAPIServerOperatorName, os.Stdout, os.Stderr))
	cmd.AddCommand(controller_operator.NewControllerOperatorCommand(controller_operator.RecommendedControllerOperatorName, os.Stdout, os.Stderr))
	cmd.AddCommand(webconsole_operator.NewWebConsoleOperatorCommand(webconsole_operator.RecommendedWebConsoleOperatorName, os.Stdout, os.Stderr))
	cmd.AddCommand(orchestration_operator.NewOrchestrationOperatorCommand(orchestration_operator.RecommendedOrchestrationOperatorName, os.Stdout, os.Stderr))

	return cmd
}
