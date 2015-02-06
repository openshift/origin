package login

import (
	"fmt"
	"os"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kubecmd "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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

			err = updateKubeconfigFile(usernameFlag, accessToken, clientCfg)
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

func updateKubeconfigFile(username, token string, clientConfig *kclient.Config) error {
	config, err := getConfigFromFile(".kubeconfig")
	if err != nil {
		return err
	}

	credentialsName := username
	if len(credentialsName) == 0 {
		credentialsName = "osc-login"
	}
	credentialsName = getUniqueName(credentialsName, getAuthInfoNames(config))
	credentials := clientcmdapi.NewAuthInfo()
	credentials.Token = token
	config.AuthInfos[credentialsName] = *credentials

	clusterName := getUniqueName("osc-login-cluster", getClusterNames(config))
	cluster := clientcmdapi.NewCluster()
	cluster.Server = clientConfig.Host
	cluster.InsecureSkipTLSVerify = clientConfig.Insecure
	cluster.CertificateAuthority = clientConfig.CAFile
	config.Clusters[clusterName] = *cluster

	contextName := getUniqueName(clusterName+"-"+credentialsName, getContextNames(config))
	context := clientcmdapi.NewContext()
	context.Cluster = clusterName
	context.AuthInfo = credentialsName
	config.Contexts[contextName] = *context

	config.CurrentContext = contextName

	err = clientcmd.WriteToFile(*config, ".kubeconfig")
	if err != nil {
		return err
	}

	return nil

}

func getAuthInfoNames(config *clientcmdapi.Config) *util.StringSet {
	ret := &util.StringSet{}
	for key := range config.AuthInfos {
		ret.Insert(key)
	}

	return ret
}

func getContextNames(config *clientcmdapi.Config) *util.StringSet {
	ret := &util.StringSet{}
	for key := range config.Contexts {
		ret.Insert(key)
	}

	return ret
}

func getClusterNames(config *clientcmdapi.Config) *util.StringSet {
	ret := &util.StringSet{}
	for key := range config.Clusters {
		ret.Insert(key)
	}

	return ret
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
