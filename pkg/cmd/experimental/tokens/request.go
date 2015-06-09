package tokens

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdRequestToken(f *clientcmd.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request-token",
		Short: "Request an access token",
		Long:  `Request an access token`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			util.CheckErr(err)

			accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, "", "")
			util.CheckErr(err)

			fmt.Printf("%v\n", string(accessToken))
		},
	}
	return cmd
}
