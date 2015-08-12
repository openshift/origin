package tokens

import (
	"io"
	"os"
	"path"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/client"

	"github.com/openshift/origin/pkg/auth/server/tokenrequest"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const (
	TokenRecommendedCommandName = "tokens"
	TOKEN_FILE_PARAM            = "token-file"
)

func NewCmdTokens(name, fullName string, f *osclientcmd.Factory, out io.Writer) *cobra.Command {
	// Parent command to which all subcommands are added.
	cmds := &cobra.Command{
		Use:   name,
		Short: "Manage authentication tokens",
		Long:  `Manage authentication tokens`,
		Run: func(c *cobra.Command, args []string) {
			c.SetOutput(os.Stdout)
			c.Help()
		},
	}

	cmds.AddCommand(NewCmdValidateToken(f))
	cmds.AddCommand(NewCmdRequestToken(f))

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
	return clientCfg.Host + path.Join("/oauth", tokenrequest.RequestTokenEndpoint)
}
