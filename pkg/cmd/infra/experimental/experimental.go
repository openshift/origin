package experimental

import (
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/experimental/tokens"
)

func NewCommandExperimental(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Run: func(c *cobra.Command, args []string) {
			// no usage
			// c.Help()
		},
	}

	cmd.AddCommand(tokens.NewCommandTokens("tokens"))

	return cmd
}
