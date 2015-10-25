package cmd

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	kclient "k8s.io/kubernetes/pkg/client/unversioned"
	kclientcmd "k8s.io/kubernetes/pkg/client/unversioned/clientcmd"
	clientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	kclientcmdapi "k8s.io/kubernetes/pkg/client/unversioned/clientcmd/api"
	"k8s.io/kubernetes/pkg/fields"
	kcmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	kubecmdconfig "k8s.io/kubernetes/pkg/kubectl/cmd/config"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/pkg/user/api"
)

const defaultClusterURL = "https://localhost:8443"

// LoginOptions is a helper for the login and setup process, gathers all information required for a
// successful login and eventual update of config files.
// Depending on the Reader present it can be interactive, asking for terminal input in
// case of any missing information.
// Notice that some methods mutate this object so it should not be reused. The Config
// provided as a pointer will also mutate (handle new auth tokens, etc).
type LoginOptions struct {
	Server      string
	CAFile      string
	InsecureTLS bool
	APIVersion  string

	// flags and printing helpers
	Username string
	Password string
	Project  string

	// infra
	StartingKubeConfig *kclientcmdapi.Config
	DefaultNamespace   string
	Config             *kclient.Config
	Reader             io.Reader
	Out                io.Writer

	// cert data to be used when authenticating
	CertFile string
	KeyFile  string

	Token string

	PathOptions *kcmdconfig.PathOptions
}

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

	if len(o.Server) == 0 {
		// we need to have a server to talk to
		if cmdutil.IsTerminal(o.Reader) {
			for !o.serverProvided() {
				defaultServer := defaultClusterURL
				promptMsg := fmt.Sprintf("Server [%s]: ", defaultServer)

				o.Server = cmdutil.PromptForStringWithDefault(o.Reader, o.Out, defaultServer, promptMsg)
			}
		}
	}

	// normalize the provided server to a format expected by config
	serverNormalized, err := config.NormalizeServerURL(o.Server)
	if err != nil {
		return nil, err
	}
	o.Server = serverNormalized
	clientConfig.Host = o.Server

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

	result := osClient.Get().AbsPath("/").Do()
	if result.Error() != nil {
		switch {
		case o.InsecureTLS:
			clientConfig.Insecure = true
			// insecure, clear CA info
			clientConfig.CAFile = ""
			clientConfig.CAData = nil

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

				clientConfig.Insecure = cmdutil.PromptForBool(os.Stdin, o.Out, "Use insecure connections? (y/n): ")
				if !clientConfig.Insecure {
					return nil, fmt.Errorf(clientcmd.GetPrettyMessageFor(result.Error()))
				}
				// insecure, clear CA info
				clientConfig.CAFile = ""
				clientConfig.CAData = nil
				fmt.Fprintln(o.Out)
			}

		default:
			return nil, result.Error()
		}
	}

	// check for matching api version
	if len(o.APIVersion) > 0 {
		clientConfig.Version = o.APIVersion
	}

	o.Config = clientConfig
	return o.Config, nil
}

// getMatchingClusters examines the kubeconfig for all clusters that point to the same server
func getMatchingClusters(clientConfig kclient.Config, kubeconfig clientcmdapi.Config) sets.String {
	ret := sets.String{}

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

	// if a token were explicitly provided, try to use it
	if o.tokenProvided() {
		clientConfig.BearerToken = o.Token
		if osClient, err := client.New(clientConfig); err == nil {
			me, err := whoAmI(osClient)
			if err == nil {
				o.Username = me.Name
				o.Config = clientConfig

				fmt.Fprintf(o.Out, "Logged into %q as %q using the token provided.\n\n", o.Config.Host, o.Username)
				return nil
			}

			if !kerrors.IsUnauthorized(err) {
				return err
			}

			fmt.Fprint(o.Out, "The token provided is invalid (probably expired).\n\n")
		}
	}

	// if a username was provided try to make use of it, but if a password were provided we force a token
	// request which will return a proper response code for that given password
	if o.usernameProvided() && !o.passwordProvided() {
		// search all valid contexts with matching server stanzas to see if we have a matching user stanza
		kubeconfig := *o.StartingKubeConfig
		matchingClusters := getMatchingClusters(*clientConfig, kubeconfig)

		for key, context := range o.StartingKubeConfig.Contexts {
			if matchingClusters.Has(context.Cluster) {
				clientcmdConfig := kclientcmd.NewDefaultClientConfig(kubeconfig, &kclientcmd.ConfigOverrides{CurrentContext: key})
				if kubeconfigClientConfig, err := clientcmdConfig.ClientConfig(); err == nil {
					if osClient, err := client.New(kubeconfigClientConfig); err == nil {
						if me, err := whoAmI(osClient); err == nil && (o.Username == me.Name) {
							clientConfig.BearerToken = kubeconfigClientConfig.BearerToken
							clientConfig.CertFile = kubeconfigClientConfig.CertFile
							clientConfig.CertData = kubeconfigClientConfig.CertData
							clientConfig.KeyFile = kubeconfigClientConfig.KeyFile
							clientConfig.KeyData = kubeconfigClientConfig.KeyData

							o.Config = clientConfig

							if key == o.StartingKubeConfig.CurrentContext {
								fmt.Fprintf(o.Out, "Logged into %q as %q using existing credentials.\n\n", o.Config.Host, o.Username)
							}

							return nil
						}
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
	fmt.Fprint(o.Out, "Login successful.\n\n")

	return nil
}

// Discover the projects available for the established session and take one to use. It
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

    $ oc new-project <projectname>

`)
		o.Project = ""

	case 1:
		o.Project = projectsItems[0].Name
		fmt.Fprintf(o.Out, "Using project %q.\n", o.Project)

	default:
		projects := sets.String{}
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

		fmt.Fprintf(o.Out, "\nYou have access to the following projects and can switch between them with 'oc project <projectname>':\n\n")
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

	newConfig, err := config.CreateConfig(o.Project, o.Config)
	if err != nil {
		return false, err
	}

	cwd, err := os.Getwd()
	if err != nil {
		return false, err
	}
	baseDir, err := cmdutil.MakeAbs(filepath.Dir(o.PathOptions.GetDefaultFilename()), cwd)
	if err != nil {
		return false, err
	}
	if err := config.RelativizeClientConfigPaths(newConfig, baseDir); err != nil {
		return false, err
	}

	configToWrite, err := config.MergeConfig(*o.StartingKubeConfig, *newConfig)
	if err != nil {
		return false, err
	}

	if err := kubecmdconfig.ModifyConfig(o.PathOptions, *configToWrite, true); err != nil {
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

func (o *LoginOptions) passwordProvided() bool {
	return len(o.Password) > 0
}

func (o *LoginOptions) serverProvided() bool {
	return (len(o.Server) > 0)
}

func (o *LoginOptions) tokenProvided() bool {
	return len(o.Token) > 0
}
