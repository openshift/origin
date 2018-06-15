package openshift_experimental

import (
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/openshift-operators/webconsole-operator"
)

func NewExperimentalCommand(out, errout io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "experimental",
		Short: "Experimental commands for OpenShift",
		Run: func(cmd *cobra.Command, args []string) {
			cmd.Help()
			os.Exit(1)
		},
	}

	cmd.AddCommand(webconsole_operator.NewWebConsoleOperatorCommand(webconsole_operator.RecommendedWebConsoleOperatorName))

	return cmd
}
