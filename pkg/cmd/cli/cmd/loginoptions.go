package cmd

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	kubecmdconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/pkg/user/api"
)

const defaultClusterURL = "https://localhost:8443"

// Gather all required information in a comprehensive order.
func (o *LoginOptions) GatherInfo() error {
	if err := o.gatherAuthInfo(); err != nil {
		return err
	}
	if err := o.gatherProjectInfo(); err != nil {
		return err
	}
	return nil
}

// getClientConfig returns back the current clientConfig as we know it.  If there is no clientConfig, it builds one with enough information
// to talk to a server.  This may involve user prompts.  This method is not threadsafe.
func (o *LoginOptions) getClientConfig() (*kclient.Config, error) {
	if o.Config != nil {
		return o.Config, nil
	}

	clientConfig := &kclient.Config{}

	// if someone specified a server, use it as the default
	if len(o.Server) > 0 {
		clientConfig.Host = o.Server

	} else {
		// we need to have a server to talk to
		if cmdutil.IsTerminal(o.Reader) {
			for !o.serverProvided() {
				defaultServer := defaultClusterURL
				promptMsg := fmt.Sprintf("OpenShift server [%s]: ", defaultServer)

				o.Server = cmdutil.PromptForStringWithDefault(o.Reader, defaultServer, promptMsg)
				clientConfig.Host = o.Server
			}
		}
	}

	if len(o.CAFile) > 0 {
		clientConfig.CAFile = o.CAFile

	} else {
		// check all cluster stanzas to see if we already have one with this URL that contains a client cert
		for _, cluster := range o.StartingKubeConfig.Clusters {
			if cluster.Server == clientConfig.Host {
				if len(cluster.CertificateAuthority) > 0 {
					clientConfig.CAFile = cluster.CertificateAuthority
					break
				}

				if len(cluster.CertificateAuthorityData) > 0 {
					clientConfig.CAData = cluster.CertificateAuthorityData
					break
				}
			}
		}

	}

	// ping to check if server is reachable
	osClient, err := client.New(clientConfig)
	if err != nil {
		return nil, err
	}

	result := osClient.Get().AbsPath("/osapi").Do()
	if result.Error() != nil {
		switch {
		case o.InsecureTLS:
			clientConfig.Insecure = true

		// certificate issue, prompt user for insecure connection
		case clientcmd.IsCertificateAuthorityUnknown(result.Error()):
			// check to see if we already have a cluster stanza that tells us to use --insecure for this particular server.  If we don't, then prompt
			clientConfigToTest := *clientConfig
			clientConfigToTest.Insecure = true
			matchingClusters := getMatchingClusters(clientConfigToTest, *o.StartingKubeConfig)

			if len(matchingClusters) > 0 {
				clientConfig.Insecure = true

			} else if cmdutil.IsTerminal(o.Reader) {
				fmt.Fprintln(o.Out, "The server uses a certificate signed by an unknown authority.")
				fmt.Fprintln(o.Out, "You can bypass the certificate check, but any data you send to the server could be intercepted by others.")

				clientConfig.Insecure = cmdutil.PromptForBool(os.Stdin, "Use insecure connections? (y/n): ")
				if !clientConfig.Insecure {
					return nil, fmt.Errorf(clientcmd.GetPrettyMessageFor(result.Error()))
				}
				fmt.Fprintln(o.Out)
			}

		default:
			return nil, result.Error()
		}
	}

	o.Config = clientConfig
	return o.Config, nil
}

// getMatchingClusters examines the kubeconfig for all clusters that point to the same server
func getMatchingClusters(clientConfig kclient.Config, kubeconfig clientcmdapi.Config) util.StringSet {
	ret := util.StringSet{}

	for key, cluster := range kubeconfig.Clusters {
		if (cluster.Server == clientConfig.Host) && (cluster.InsecureSkipTLSVerify == clientConfig.Insecure) && (cluster.CertificateAuthority == clientConfig.CAFile) && (bytes.Compare(cluster.CertificateAuthorityData, clientConfig.CAData) == 0) {
			ret.Insert(key)
		}
	}

	return ret
}

// Negotiate a bearer token with the auth server, or try to reuse one based on the
// information already present. In case of any missing information, ask for user input
// (usually username and password, interactive depending on the Reader).
func (o *LoginOptions) gatherAuthInfo() error {
	directClientConfig, err := o.getClientConfig()
	if err != nil {
		return err
	}

	// make a copy and use it to avoid mutating the original
	t := *directClientConfig
	clientConfig := &t

	// make sure we have a username before continuing
	if !o.usernameProvided() {
		if cmdutil.IsTerminal(o.Reader) {
			for !o.usernameProvided() {
				o.Username = cmdutil.PromptForString(o.Reader, "Username: ")
			}
		}
	}

	// search all valid contexts with matching server stanzas to see if we have a matching user stanza
	kubeconfig := *o.StartingKubeConfig
	matchingClusters := getMatchingClusters(*clientConfig, kubeconfig)

	for key, context := range o.StartingKubeConfig.Contexts {
		if matchingClusters.Has(context.Cluster) {
			clientcmdConfig := kclientcmd.NewDefaultClientConfig(kubeconfig, &kclientcmd.ConfigOverrides{CurrentContext: key})
			if kubeconfigClientConfig, err := clientcmdConfig.ClientConfig(); err == nil {
				if osClient, err := client.New(kubeconfigClientConfig); err == nil {
					if me, err := whoAmI(osClient); err == nil && (o.Username == me.Name) {
						o.Config.BearerToken = kubeconfigClientConfig.BearerToken
						o.Config.CertFile = kubeconfigClientConfig.CertFile
						o.Config.CertData = kubeconfigClientConfig.CertData
						o.Config.KeyFile = kubeconfigClientConfig.KeyFile
						o.Config.KeyData = kubeconfigClientConfig.KeyData

						if key == o.StartingKubeConfig.CurrentContext {
							fmt.Fprintf(o.Out, "Already logged into %q as %q.\n", o.Config.Host, o.Username)
						}
						return nil
					}
				}
			}
		}
	}

	// if kubeconfig doesn't already have a matching user stanza...
	clientConfig.BearerToken = ""
	clientConfig.CertData = []byte{}
	clientConfig.KeyData = []byte{}
	clientConfig.CertFile = o.CertFile
	clientConfig.KeyFile = o.KeyFile
	token, err := tokencmd.RequestToken(o.Config, o.Reader, o.Username, o.Password)
	if err != nil {
		return err
	}
	clientConfig.BearerToken = token

	osClient, err := client.New(clientConfig)
	if err != nil {
		return err
	}

	me, err := whoAmI(osClient)
	if err != nil {
		return err
	}
	o.Username = me.Name
	o.Config = clientConfig
	fmt.Fprintln(o.Out, "Login successful.\n")

	return nil
}

// Discover the projects available for the stabilished session and take one to use. It
// fails in case of no existing projects, and print out useful information in case of
// multiple projects.
// Requires o.Username to be set.
func (o *LoginOptions) gatherProjectInfo() error {
	me, err := o.whoAmI()
	if err != nil {
		return err
	}

	if o.Username != me.Name {
		return fmt.Errorf("current user, %v, does not match expected user %v", me.Name, o.Username)
	}

	oClient, err := client.New(o.Config)
	if err != nil {
		return err
	}

	projects, err := oClient.Projects().List(labels.Everything(), fields.Everything())
	if err != nil {
		return err
	}

	projectsItems := projects.Items

	switch len(projectsItems) {
	case 0:
		fmt.Fprintf(o.Out, `You don't have any projects. You can try to create a new project, by running

    $ osc new-project <projectname>

`)

	case 1:
		o.Project = projectsItems[0].Name
		fmt.Fprintf(o.Out, "Using project %q.\n", o.Project)

	default:
		projects := util.StringSet{}
		for _, project := range projectsItems {
			projects.Insert(project.Name)
		}

		namespace := o.DefaultNamespace
		if !projects.Has(namespace) {
			if namespace != kapi.NamespaceDefault && projects.Has(kapi.NamespaceDefault) {
				namespace = kapi.NamespaceDefault
			} else {
				namespace = projects.List()[0]
			}
		}

		if current, err := oClient.Projects().Get(namespace); err == nil {
			o.Project = current.Name
			fmt.Fprintf(o.Out, "Using project %q.\n", o.Project)
		} else if !kerrors.IsNotFound(err) && !clientcmd.IsForbidden(err) {
			return err
		}

		fmt.Fprintf(o.Out, "\nYou have access to the following projects and can switch between them with 'osc project <projectname>':\n\n")
		for _, p := range projects.List() {
			if o.Project == p {
				fmt.Fprintf(o.Out, "  * %s (current)\n", p)
			} else {
				fmt.Fprintf(o.Out, "  * %s\n", p)
			}
		}
		fmt.Fprintln(o.Out)
	}

	return nil
}

// Save all the information present in this helper to a config file. An explicit config
// file path can be provided, if not use the established conventions about config
// loading rules. Will create a new config file if one can't be found at all. Will only
// succeed if all required info is present.
func (o *LoginOptions) SaveConfig() (bool, error) {
	if len(o.Username) == 0 {
		return false, fmt.Errorf("Insufficient data to merge configuration.")
	}

	globalExistedBefore := true
	if _, err := os.Stat(o.PathOptions.GlobalFile); os.IsNotExist(err) {
		globalExistedBefore = false
	}

	newConfig := config.CreateConfig(o.Username, o.Project, o.Config)
	baseDir := filepath.Dir(o.PathOptions.GetDefaultFilename())
	if err := config.RelativizeClientConfigPaths(&newConfig, baseDir); err != nil {
		return false, err
	}

	configToWrite, err := config.MergeConfig(*o.StartingKubeConfig, newConfig)
	if err != nil {
		return false, err
	}

	if err := kubecmdconfig.ModifyConfig(o.PathOptions, *configToWrite); err != nil {
		return false, err
	}

	created := false
	if _, err := os.Stat(o.PathOptions.GlobalFile); err == nil {
		created = created || !globalExistedBefore
	}

	return created, nil
}

func whoAmI(client *client.Client) (*api.User, error) {
	me, err := client.Users().Get("~")
	if err != nil {
		return nil, err
	}

	return me, nil
}

func (o LoginOptions) whoAmI() (*api.User, error) {
	client, err := client.New(o.Config)
	if err != nil {
		return nil, err
	}

	return whoAmI(client)
}

func (o *LoginOptions) usernameProvided() bool {
	return len(o.Username) > 0
}

func (o *LoginOptions) serverProvided() bool {
	return (len(o.Server) > 0)
}
