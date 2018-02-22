package login

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	kterm "k8s.io/kubernetes/pkg/kubectl/util/term"

	"github.com/openshift/origin/pkg/client/config"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/term"
	"github.com/openshift/origin/pkg/oc/cli/cmd/errors"
	loginutil "github.com/openshift/origin/pkg/oc/cli/cmd/login/util"
	cliconfig "github.com/openshift/origin/pkg/oc/cli/config"
	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	cmderr "github.com/openshift/origin/pkg/oc/errors"
	"github.com/openshift/origin/pkg/oc/util/tokencmd"
	projectclient "github.com/openshift/origin/pkg/project/generated/internalclientset"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
)

const defaultClusterURL = "https://localhost:8443"

const projectsItemsSuppressThreshold = 50

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

	// flags and printing helpers
	Username string
	Password string
	Project  string

	// infra
	StartingKubeConfig *kclientcmdapi.Config
	DefaultNamespace   string
	Config             *restclient.Config
	Reader             io.Reader
	Out                io.Writer
	ErrOut             io.Writer

	// cert data to be used when authenticating
	CertFile string
	KeyFile  string

	Token string

	PathOptions *kclientcmd.PathOptions

	CommandName    string
	RequestTimeout time.Duration
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
func (o *LoginOptions) getClientConfig() (*restclient.Config, error) {
	if o.Config != nil {
		return o.Config, nil
	}

	if len(o.Server) == 0 {
		// we need to have a server to talk to
		if kterm.IsTerminal(o.Reader) {
			for !o.serverProvided() {
				defaultServer := defaultClusterURL
				promptMsg := fmt.Sprintf("Server [%s]: ", defaultServer)
				o.Server = term.PromptForStringWithDefault(o.Reader, o.Out, defaultServer, promptMsg)
			}
		}
	}

	clientConfig := &restclient.Config{}

	// ensure clientConfig has timeout option
	if o.RequestTimeout > 0 {
		clientConfig.Timeout = o.RequestTimeout
	}

	// normalize the provided server to a format expected by config
	serverNormalized, err := config.NormalizeServerURL(o.Server)
	if err != nil {
		return nil, err
	}
	o.Server = serverNormalized
	clientConfig.Host = o.Server

	// use specified CA or find existing CA
	if len(o.CAFile) > 0 {
		clientConfig.CAFile = o.CAFile
		clientConfig.CAData = nil
	} else if caFile, caData, ok := findExistingClientCA(clientConfig.Host, *o.StartingKubeConfig); ok {
		clientConfig.CAFile = caFile
		clientConfig.CAData = caData
	}

	// try to TCP connect to the server to make sure it's reachable, and discover
	// about the need of certificates or insecure TLS
	if err := dialToServer(*clientConfig); err != nil {
		switch err.(type) {
		// certificate authority unknown, check or prompt if we want an insecure
		// connection or if we already have a cluster stanza that tells us to
		// connect to this particular server insecurely
		case x509.UnknownAuthorityError, x509.HostnameError, x509.CertificateInvalidError:
			if o.InsecureTLS ||
				hasExistingInsecureCluster(*clientConfig, *o.StartingKubeConfig) ||
				promptForInsecureTLS(o.Reader, o.Out, err) {
				clientConfig.Insecure = true
				clientConfig.CAFile = ""
				clientConfig.CAData = nil
			} else {
				return nil, clientcmd.GetPrettyErrorForServer(err, o.Server)
			}
		// TLS record header errors, like oversized record which usually means
		// the server only supports "http"
		case tls.RecordHeaderError:
			return nil, clientcmd.GetPrettyErrorForServer(err, o.Server)
		default:
			if _, ok := err.(*net.OpError); ok {
				return nil, fmt.Errorf("%v - verify you have provided the correct host and port and that the server is currently running.", err)
			}
			return nil, err
		}
	}

	o.Config = clientConfig

	return o.Config, nil
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
		if me, err := whoAmI(clientConfig); err == nil {
			o.Username = me.Name
			o.Config = clientConfig

			fmt.Fprintf(o.Out, "Logged into %q as %q using the token provided.\n\n", o.Config.Host, o.Username)
			return nil

		} else {
			if kerrors.IsUnauthorized(err) {
				return fmt.Errorf("The token provided is invalid or expired.\n\n")
			}

			return err
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
					if me, err := whoAmI(kubeconfigClientConfig); err == nil && (o.Username == me.Name) {
						clientConfig.BearerToken = kubeconfigClientConfig.BearerToken
						clientConfig.CertFile = kubeconfigClientConfig.CertFile
						clientConfig.CertData = kubeconfigClientConfig.CertData
						clientConfig.KeyFile = kubeconfigClientConfig.KeyFile
						clientConfig.KeyData = kubeconfigClientConfig.KeyData

						o.Config = clientConfig

						fmt.Fprintf(o.Out, "Logged into %q as %q using existing credentials.\n\n", o.Config.Host, o.Username)

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

	me, err := whoAmI(clientConfig)
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

	projectClient, err := projectclient.NewForConfig(o.Config)
	if err != nil {
		return err
	}

	projectsList, err := projectClient.Project().Projects().List(metav1.ListOptions{})
	// if we're running on kube (or likely kube), just set it to "default"
	if kerrors.IsNotFound(err) || kerrors.IsForbidden(err) {
		fmt.Fprintf(o.Out, "Using \"default\".  You can switch projects with:\n\n '%s project <projectname>'\n", o.CommandName)
		o.Project = "default"
		return nil
	}
	if err != nil {
		return err
	}

	projectsItems := projectsList.Items
	projects := sets.String{}
	for _, project := range projectsItems {
		projects.Insert(project.Name)
	}

	if len(o.DefaultNamespace) > 0 && !projects.Has(o.DefaultNamespace) {
		// Attempt a direct get of our current project in case it hasn't appeared in the list yet
		if currentProject, err := projectClient.Project().Projects().Get(o.DefaultNamespace, metav1.GetOptions{}); err == nil {
			// If we get it successfully, add it to the list
			projectsItems = append(projectsItems, *currentProject)
			projects.Insert(currentProject.Name)
		}
	}

	switch len(projectsItems) {
	case 0:
		canRequest, err := loginutil.CanRequestProjects(o.Config, o.DefaultNamespace)
		if err != nil {
			return err
		}
		msg := errors.NoProjectsExistMessage(canRequest, o.CommandName)
		fmt.Fprintf(o.Out, msg)
		o.Project = ""

	case 1:
		o.Project = projectsItems[0].Name
		fmt.Fprintf(o.Out, "You have one project on this server: %q\n\n", o.Project)
		fmt.Fprintf(o.Out, "Using project %q.\n", o.Project)

	default:
		namespace := o.DefaultNamespace
		if !projects.Has(namespace) {
			if namespace != metav1.NamespaceDefault && projects.Has(metav1.NamespaceDefault) {
				namespace = metav1.NamespaceDefault
			} else {
				namespace = projects.List()[0]
			}
		}

		current, err := projectClient.Project().Projects().Get(namespace, metav1.GetOptions{})
		if err != nil && !kerrors.IsNotFound(err) && !clientcmd.IsForbidden(err) {
			return err
		}
		o.Project = current.Name

		// Suppress project listing if the number of projects available to the user is greater than the threshold. Prevents unnecessarily noisy logins on clusters with large numbers of projects
		if len(projectsItems) > projectsItemsSuppressThreshold {
			fmt.Fprintf(o.Out, "You have access to %d projects, the list has been suppressed. You can list all projects with '%s projects'\n\n", len(projectsItems), o.CommandName)
		} else {
			fmt.Fprintf(o.Out, "You have access to the following projects and can switch between them with '%s project <projectname>':\n\n", o.CommandName)
			for _, p := range projects.List() {
				if o.Project == p {
					fmt.Fprintf(o.Out, "  * %s\n", p)
				} else {
					fmt.Fprintf(o.Out, "    %s\n", p)
				}
			}
			fmt.Fprintln(o.Out)
		}
		fmt.Fprintf(o.Out, "Using project %q.\n", o.Project)
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

	newConfig, err := cliconfig.CreateConfig(o.Project, o.Config)
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
	if err := cliconfig.RelativizeClientConfigPaths(newConfig, baseDir); err != nil {
		return false, err
	}

	configToWrite, err := cliconfig.MergeConfig(*o.StartingKubeConfig, *newConfig)
	if err != nil {
		return false, err
	}

	if err := kclientcmd.ModifyConfig(o.PathOptions, *configToWrite, true); err != nil {
		if !os.IsPermission(err) {
			return false, err
		}

		out := &bytes.Buffer{}
		cmderr.PrintError(errors.ErrKubeConfigNotWriteable(o.PathOptions.GetDefaultFilename(), o.PathOptions.IsExplicitFile(), err), out)
		return false, fmt.Errorf("%v", out)
	}

	created := false
	if _, err := os.Stat(o.PathOptions.GlobalFile); err == nil {
		created = created || !globalExistedBefore
	}

	return created, nil
}

func (o LoginOptions) whoAmI() (*userapi.User, error) {
	return whoAmI(o.Config)
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
