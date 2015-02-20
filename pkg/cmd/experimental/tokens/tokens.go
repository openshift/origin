package tokens

import (
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	"github.com/openshift/origin/pkg/cmd/server/origin"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	TOKEN_FILE_PARAM = "token-file"
)

func NewCmdTokens(f *osclientcmd.Factory, parentName, name string) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "manage authentication tokens",
		Long:  `manage authentication tokens`,
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	cmds.AddCommand(NewCmdValidateToken(f))
	cmds.AddCommand(NewCmdRequestToken(f))
	cmds.AddCommand(NewCmdWhoAmI(f))

	return cmds
}

func getFlagString(cmd *cobra.Command, flag string) string {
	f := cmd.Flags().Lookup(flag)
	if f == nil {
		glog.Fatalf("Flag accessed but not defined for command %s: %s", cmd.Name(), flag)
	}
	return f.Value.String()
}

func getRequestTokenURL(clientCfg *client.Config) string {
	return clientCfg.Host + origin.OpenShiftLoginPrefix + tokenrequest.RequestTokenEndpoint
}
