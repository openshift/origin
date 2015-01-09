package login

import (
	"fmt"
	"os"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func NewCmdLogin(name string, parent *cobra.Command) *cobra.Command {
	clientConfig := defaultClientConfig(parent.PersistentFlags())
	f := cmd.NewFactory(clientConfig)
	f.BindFlags(parent.PersistentFlags())

	cmds := &cobra.Command{
		Use:   name,
		Short: "Logs in and returns a session token",
		Long: `Logs in to the OpenShift server and prints out a session token.

Username and password can be provided through flags, the command will 
prompt for user input if not provided.
`,
		Run: func(cmd *cobra.Command, args []string) {
			clientCfg, err := f.OpenShiftClientConfig.ClientConfig()
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			usernameFlag := kubecmd.GetFlagString(cmd, "username")
			passwordFlag := kubecmd.GetFlagString(cmd, "password")

			accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, usernameFlag, passwordFlag)
			if err != nil {
				glog.Fatalf("%v\n", err)
			}

			fmt.Printf("Auth token: %v\n", string(accessToken))
		},
	}

	cmds.Flags().StringP("username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringP("password", "p", "", "Password, will prompt if not provided")
	return cmds
}

// Copy of kubectl/cmd/DefaultClientConfig, using NewNonInteractiveDeferredLoadingClientConfig
// TODO find and merge duplicates, this is also in other places
func defaultClientConfig(flags *pflag.FlagSet) clientcmd.ClientConfig {
	loadingRules := clientcmd.NewClientConfigLoadingRules()
	loadingRules.EnvVarPath = os.Getenv(clientcmd.RecommendedConfigPathEnvVar)
	flags.StringVar(&loadingRules.CommandLinePath, "kubeconfig", "", "Path to the kubeconfig file to use for CLI requests.")

	overrides := &clientcmd.ConfigOverrides{}
	overrideFlags := clientcmd.RecommendedConfigOverrideFlags("")
	overrideFlags.ContextOverrideFlags.NamespaceShort = "n"
	clientcmd.BindOverrideFlags(overrides, flags, overrideFlags)
	clientConfig := clientcmd.NewNonInteractiveDeferredLoadingClientConfig(loadingRules, overrides)

	return clientConfig
}
