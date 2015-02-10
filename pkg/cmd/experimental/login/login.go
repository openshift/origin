package login

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/cmd"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
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

			username := ""

			// check to see if we're already signed in.  If so, simply make sure that .kubeconfig has that information
			if userFullName, err := whoami(clientCfg); err == nil {
				if err := updateKubeconfigFile(userFullName, clientCfg.BearerToken, f.OpenShiftClientConfig); err != nil {
					glog.Fatalf("%v\n", err)
				}
				username = userFullName

			} else {
				usernameFlag := kubecmd.GetFlagString(cmd, "username")
				passwordFlag := kubecmd.GetFlagString(cmd, "password")

				accessToken, err := tokencmd.RequestToken(clientCfg, os.Stdin, usernameFlag, passwordFlag)
				if err != nil {
					glog.Fatalf("%v\n", err)
				}

				err = updateKubeconfigFile(usernameFlag, accessToken, f.OpenShiftClientConfig)
				if err != nil {
					glog.Fatalf("%v\n", err)
				}

				username = usernameFlag
			}

			fmt.Printf("Logged into %v as %v\n", clientCfg.Host, username)
		},
	}

	cmds.Flags().StringP("username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringP("password", "p", "", "Password, will prompt if not provided")
	return cmds
}

func whoami(clientCfg *kclient.Config) (string, error) {
	osClient, err := client.New(clientCfg)
	if err != nil {
		return "", err
	}

	me, err := osClient.Users().Get("~")
	if err != nil {
		return "", err
	}

	return me.FullName, nil
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

func updateKubeconfigFile(username, token string, clientCfg clientcmd.ClientConfig) error {
	rawMergedConfig, err := clientCfg.RawConfig()
	if err != nil {
		return err
	}
	clientConfig, err := clientCfg.ClientConfig()
	if err != nil {
		return err
	}
	namespace, err := clientCfg.Namespace()
	if err != nil {
		return err
	}

	config := clientcmdapi.NewConfig()

	credentialsName := username
	if len(credentialsName) == 0 {
		credentialsName = "osc-login"
	}
	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = token
	config.AuthInfos[credentialsName] = *credentials

	serverAddr := flagtypes.Addr{Value: clientConfig.Host}.Default()
	clusterName := fmt.Sprintf("%v:%v", serverAddr.Host, serverAddr.Port)
	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientConfig.Host
	cluster.InsecureSkipTLSVerify = clientConfig.Insecure
	cluster.CertificateAuthority = clientConfig.CAFile
	config.Clusters[clusterName] = *cluster

	contextName := clusterName + "-" + credentialsName
	context := clientcmdapi.NewContext()
	context.Cluster = clusterName
	context.AuthInfo = credentialsName
	context.Namespace = namespace
	config.Contexts[contextName] = *context

	config.CurrentContext = contextName

	configToModify, err := getConfigFromFile(".kubeconfig")
	if err != nil {
		return err
	}

	configToWrite, err := MergeConfig(rawMergedConfig, *configToModify, *config)
	if err != nil {
		return err
	}
	err = clientcmd.WriteToFile(*configToWrite, ".kubeconfig")
	if err != nil {
		return err
	}

	return nil

}

func getConfigFromFile(filename string) (*clientcmdapi.Config, error) {
	var err error
	config, err := clientcmd.LoadFromFile(filename)
	if err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	if config == nil {
		config = clientcmdapi.NewConfig()
	}

	return config, nil
}

func getUniqueName(basename string, existingNames *util.StringSet) string {
	if !existingNames.Has(basename) {
		return basename
	}

	for i := 0; i < 100; i++ {
		trialName := fmt.Sprintf("%v-%d", basename, i)
		if !existingNames.Has(trialName) {
			return trialName
		}
	}

	return string(util.NewUUID())
}
