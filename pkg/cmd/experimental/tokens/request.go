package tokens

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
)

func NewCmdRequestToken(clientCfg *clientcmd.Config) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "request-token",
		Short: "request an access token",
		Long:  `request an access token`,
		Run: func(cmd *cobra.Command, args []string) {
			accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin)
			if err != nil {
				fmt.Printf("%v\n", err)
				return
			}

			fmt.Printf("%v\n", string(accessToken))
		},
	}
	return cmd
}
