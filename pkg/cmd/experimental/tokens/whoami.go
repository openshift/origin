package tokens

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdWhoAmI(clientCfg *clientcmd.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "checks the identity associated with an access token",
		Long:  `checks the identity associated with an access token`,
		Run: func(cmd *cobra.Command, args []string) {
			token := ""
			if cmd.Flags().Lookup("token") != nil {
				token = getFlagString(cmd, "token")
			}
			whoami(token, clientCfg)

		},
	}
	cmd.Flags().String("token", "", "Token value")
	return cmd
}

func whoami(token string, clientCfg *clientcmd.Config) {
	osCfg := clientCfg.OpenShiftConfig()
	// This will be pulled out of the auth config file after https://github.com/GoogleCloudPlatform/kubernetes/pull/2437
	if len(token) > 0 {
		osCfg.BearerToken = token
	}

	osClient, err := osclient.New(osCfg)
	if err != nil {
		fmt.Printf("Error building osClient: %v\n", err)
		return
	}

	me, err := osClient.Users().Get("~")
	if err != nil {
		// let's pretend that we can determine that we got back a 401.  we need an updated kubernetes for this
		accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin)
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}

		osCfg := clientCfg.OpenShiftConfig()
		osCfg.BearerToken = accessToken
		osClient, _ = osclient.New(osCfg)

		me, err = osClient.Users().Get("~")
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			return
		}
	}

	fmt.Printf("%v\n", me)
}
