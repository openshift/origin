package tokens

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdRequestToken(f *clientcmd.Factory) *cobra.Command {
	username := ""
	password := ""
	cmd := &cobra.Command{
		Use:   "request-token",
		Short: "Request an access token",
		Long:  `Request an access token`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			util.CheckErr(err)

			accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, username, password)
			util.CheckErr(err)

			fmt.Printf("%v\n", string(accessToken))
		},
	}
	cmd.Flags().StringVarP(&username, "username", "u", "", "Username, will prompt if not provided")
	cmd.Flags().StringVarP(&password, "password", "p", "", "Password, will prompt if not provided")
	return cmd
}
