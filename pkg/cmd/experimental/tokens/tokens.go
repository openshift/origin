package tokens

import (
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	TOKEN_FILE_PARAM = "token-file"
)

func NewCommandTokens(name string) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "manage authentication tokens",
		Long:  `manage authentication tokens`,
		Run:   runHelp,
	}

	// copied out of kubernetes kubectl so that I'll be ready when that and osc finally merge in
	// Globally persistent flags across all subcommands.
	// TODO Change flag names to consts to allow safer lookup from subcommands.
	// TODO Add a verbose flag that turns on glog logging. Probably need a way
	// to do that automatically for every subcommand.
	clientCfg := clientcmd.NewConfig()
	clientCfg.Bind(cmds.PersistentFlags())

	cmds.AddCommand(NewCmdValidateToken(clientCfg))
	cmds.AddCommand(NewCmdRequestToken(clientCfg))
	cmds.AddCommand(NewCmdWhoAmI(clientCfg))

	return cmds
}

func runHelp(cmd *cobra.Command, args []string) {
	cmd.Help()
}

func getFlagString(cmd *cobra.Command, flag string) string {
	f := cmd.Flags().Lookup(flag)
	if f == nil {
		glog.Fatalf("Flag accessed but not defined for command %s: %s", cmd.Name(), flag)
	}
	return f.Value.String()
}

func getRequestTokenURL(clientCfg *clientcmd.Config) string {
	return clientCfg.KubeConfig().Host + origin.OpenShiftLoginPrefix + tokenrequest.RequestTokenEndpoint
}
