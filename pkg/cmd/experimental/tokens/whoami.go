package tokens

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdWhoAmI(f *cmd.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "whoami",
		Short: "checks the identity associated with an access token",
		Long:  `checks the identity associated with an access token`,
		Run: func(cmd *cobra.Command, args []string) {
			token := ""
			if cmd.Flags().Lookup("token") != nil {
				token = getFlagString(cmd, "token")
			}

			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				fmt.Errorf("%v\n", err)
			}

			whoami(token, clientCfg, cmd)

		},
	}
	cmd.Flags().String("token", "", "Token value")
	return cmd
}

func whoami(token string, clientCfg *kclient.Config, cmd *cobra.Command) {
	// TODO this is now pulled out of the auth config file (https://github.com/GoogleCloudPlatform/kubernetes/pull/2437)
	if len(token) > 0 {
		clientCfg.BearerToken = token
	}

	osClient, err := osclient.New(clientCfg)
	if err != nil {
		fmt.Printf("Error building osClient: %v\n", err)
		return
	}

	me, err := osClient.Users().Get("~")
	if err != nil {
		glog.Errorf("Error fetching user: %v\n", err)

		// let's pretend that we can determine that we got back a 401.  we need an updated kubernetes for this
		accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, "", "")
		if err != nil {
			fmt.Printf("%v\n", err)
			return
		}

		clientCfg.BearerToken = accessToken
		osClient, _ = osclient.New(clientCfg)

		me, err = osClient.Users().Get("~")
		if err != nil {
			fmt.Printf("Error making request: %v\n", err)
			return
		}
	}

	fmt.Printf("%v\n", me)
}
