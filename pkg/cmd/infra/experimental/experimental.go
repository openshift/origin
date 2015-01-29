package experimental

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	"github.com/openshift/origin/pkg/cmd/experimental/tokens"
)

func NewCommandExperimental(name string) *cobra.Command {
	cmd := &cobra.Command{
		Use: name,
		Run: func(c *cobra.Command, args []string) {
			c.Help()
		},
	}

	out := os.Stdout

	cmd.AddCommand(tokens.NewCommandTokens("tokens"))

	cmdconfig := config.NewCmdConfig(out)
	cmdconfig.Long = fmt.Sprintf(`Manages .kubeconfig files using subcommands like:

%[1]s config use-context my-context
%[1]s config set preferences.some true

Reference: https://github.com/GoogleCloudPlatform/kubernetes/blob/master/docs/kubeconfig-file.md`, name)
	cmd.AddCommand(cmdconfig)

	return cmd
}
