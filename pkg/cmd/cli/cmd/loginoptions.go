package cmd

import (
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmd "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd"
	clientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/util"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	"github.com/openshift/origin/pkg/cmd/util/tokencmd"
	"github.com/openshift/origin/pkg/user/api"
)

const defaultClusterURL = "https://localhost:8443"

// Helper for the login and setup process, gathers all information required for a
// successful login and eventual update of config files.
// Depending on the Reader present it can be interactive, asking for terminal input in
// case of any missing information.
// Notice that some methods mutate this object so it should not be reused. The Config
// provided as a pointer will also mutate (handle new auth tokens, etc).
type LoginOptions struct {
	// flags and printing helpers
	Username string
	Password string
	Project  string

	// infra
	ClientConfig kclientcmd.ClientConfig
	Config       *kclient.Config
	Reader       io.Reader

	// flow controllers
	gatherServerInfo  bool
	gatherAuthInfo    bool
	gatherProjectInfo bool

	// Optional, if provided will only try to save in it
	PathToSaveConfig string
}

// Gather all required information in a comprehensive order.
func (o *LoginOptions) GatherInfo() error {
	if err := o.GatherServerInfo(); err != nil {
		return err
	}
	if err := o.GatherAuthInfo(); err != nil {
		return err
	}
	if err := o.GatherProjectInfo(); err != nil {
		return err
	}
	return nil
}

// Makes sure it has all the needed information about the server we are connecting to,
// particularly the host address and certificate information. For every information not
// present ask for interactive user input. Will also ping the server to make sure we can
// connect to it, and if any problem is found (e.g. certificate issues), ask the user about
// connecting insecurely.
func (o *LoginOptions) GatherServerInfo() error {
	// we need to have a server to talk to

	if util.IsTerminal(o.Reader) {
		for !o.serverProvided() {
			defaultServer := defaultClusterURL
			promptMsg := fmt.Sprintf("Please provide the server URL or just <enter> to use '%v': ", defaultServer)

			server := util.PromptForStringWithDefault(o.Reader, defaultServer, promptMsg)
			kclientcmd.DefaultCluster = clientcmdapi.Cluster{Server: server}
		}
	}

	// we know the server we are expected to use

	clientCfg, err := o.ClientConfig.ClientConfig()
	if err != nil {
		return err
	}

	// ping to check if server is reachable

	osClient, err := client.New(clientCfg)
	if err != nil {
		return err
	}

	result := osClient.Get().AbsPath("/osapi").Do()
	if result.Error() != nil {
		// certificate issue, prompt user for insecure connection

		if clientcmd.IsCertificateAuthorityUnknown(result.Error()) {
			fmt.Println("The server uses a certificate signed by unknown authority. You can bypass the certificate check but it will make all connections insecure.")

			clientCfg.Insecure = util.PromptForBool(os.Stdin, "Use insecure connections (strongly discouraged)? [y/N] ")
			if !clientCfg.Insecure {
				return fmt.Errorf(clientcmd.GetPrettyMessageFor(result.Error()))
			}
		}
	}

	// we have all info we need, now we can have a proper Config

	o.Config = clientCfg

	o.gatherServerInfo = true
	return nil
}

// Negotiate a bearer token with the auth server, or try to reuse one based on the
// information already present. In case of any missing information, ask for user input
// (usually username and password, interactive depending on the Reader).
func (o *LoginOptions) GatherAuthInfo() error {
	if err := o.assertGatheredServerInfo(); err != nil {
		return err
	}

	if me, err := o.Whoami(); err == nil && (!o.usernameProvided() || (o.usernameProvided() && o.Username == usernameFromUser(me))) {
		o.Username = usernameFromUser(me)
		fmt.Printf("Already logged into '%v' as '%v'.\n", o.Config.Host, o.Username)

	} else {
		// if not, we need to log in again

		o.Config.BearerToken = ""
		token, err := tokencmd.RequestToken(o.Config, o.Reader, o.Username, o.Password)
		if err != nil {
			return err
		}
		o.Config.BearerToken = token

		me, err := o.Whoami()
		if err != nil {
			return err
		}
		o.Username = usernameFromUser(me)
		fmt.Printf("Logged into '%v' as '%v'.\n", o.Config.Host, o.Username)
	}

	// TODO investigate about the safety and intent of the proposal below
	// if trying to log in an user that's not the currently logged in, try to reuse an existing token

	// if o.usernameProvided() {
	// 	glog.V(5).Infof("Checking existing authentication info for '%v'...\n", o.Username)

	// 	for _, ctx := range rawCfg.Contexts {
	// 		authInfo := rawCfg.AuthInfos[ctx.AuthInfo]
	// 		clusterInfo := rawCfg.Clusters[ctx.Cluster]

	// 		if ctx.AuthInfo == o.Username && clusterInfo.Server == o.Server && len(authInfo.Token) > 0 { // only token for now
	// 			glog.V(5).Infof("Authentication exists for '%v' on '%v', trying to use it...\n", o.Server, o.Username)

	// 			o.Config.BearerToken = authInfo.Token

	// 			if me, err := whoami(o.Config); err == nil && usernameFromUser(me) == o.Username {
	// 				o.Username = usernameFromUser(me)
	// 				return nil
	// 			}

	// 			glog.V(5).Infof("Token %v no longer valid for '%v', can't use it\n", authInfo.Token, o.Username)
	// 		}
	// 	}
	// }

	o.gatherAuthInfo = true
	return nil
}

// Discover the projects available for the stabilished session and take one to use. It
// fails in case of no existing projects, and print out useful information in case of
// multiple projects.
func (o *LoginOptions) GatherProjectInfo() error {
	if err := o.assertGatheredAuthInfo(); err != nil {
		return err
	}

	oClient, err := client.New(o.Config)
	if err != nil {
		return err
	}

	projects, err := oClient.Projects().List(labels.Everything(), labels.Everything())
	if err != nil {
		return err
	}

	projectsItems := projects.Items

	switch len(projectsItems) {
	case 0:
		me, err := o.Whoami()
		if err != nil {
			return err
		}
		// TODO most users will not be allowed to run the suggested commands below, so we should check it and/or
		// have a server endpoint that allows an admin to describe to users how to request projects
		fmt.Printf(`You don't have any project.
If you have access to create a new project, run 'openshift ex new-project <projectname> --admin=%s'.
To be added as an admin to an existing project, run 'openshift ex policy add-user admin %s -n <projectname>'.
`, me.Name, me.Name)

	case 1:
		o.Project = projectsItems[0].Name
		fmt.Printf("Using project '%v'.\n", o.Project)

	default:
		projects := []string{}

		for _, project := range projectsItems {
			projects = append(projects, project.Name)
		}

		namespace, err := o.ClientConfig.Namespace()
		if err != nil {
			return err
		}

		if current, err := oClient.Projects().Get(namespace); err != nil {
			if kerrors.IsNotFound(err) || clientcmd.IsForbidden(err) {
				o.Project = projects[0]
			} else {
				return err
			}
		} else {
			o.Project = current.Name
		}

		if n := len(projects); n > 10 {
			projects = projects[:10]
			fmt.Printf("You have %d projects, displaying only the first 10. To view all your projects run 'osc get projects'.\n", n)
		}
		var sortedProjects sort.StringSlice = projects
		sortedProjects.Sort()
		fmt.Printf("Your projects are: %v. You can switch between them at any time using 'osc project <project-name>'.\n", strings.Join(projects, ", "))
		fmt.Printf("Using project '%v'.\n", o.Project)
	}

	o.gatherProjectInfo = true
	return nil
}

// Save all the information present in this helper to a config file. An explicit config
// file path can be provided, if not use the established conventions about config
// loading rules. Will create a new config file if one can't be found at all. Will only
// succeed if all required info is present.
func (o *LoginOptions) SaveConfig() (created bool, err error) {
	if len(o.Username) == 0 {
		return false, fmt.Errorf("Insufficient data to merge configuration.")
	}

	var configStore *config.ConfigStore

	if len(o.PathToSaveConfig) > 0 {
		configStore, err = config.LoadFrom(o.PathToSaveConfig)
		if err != nil {
			return created, err
		}
	} else {
		configStore, err = config.LoadWithLoadingRules()
		if err != nil {
			configStore, err = config.CreateEmpty()
			if err != nil {
				return created, err
			}
			created = true
		}
	}

	rawCfg, err := o.ClientConfig.RawConfig()
	if err != nil {
		return created, err
	}
	return created, configStore.SaveToFile(o.Username, o.Project, o.Config, rawCfg)
}

func (o *LoginOptions) Whoami() (*api.User, error) {
	oClient, err := client.New(o.Config)
	if err != nil {
		return nil, err
	}

	me, err := oClient.Users().Get("~")
	if err != nil {
		return nil, err
	}

	return me, nil
}

func (o *LoginOptions) assertGatheredServerInfo() error {
	if !o.gatherServerInfo {
		return fmt.Errorf("Must gather server info first.")
	}
	return nil
}

func (o *LoginOptions) assertGatheredAuthInfo() error {
	if !o.gatherAuthInfo {
		return fmt.Errorf("Must gather auth info first.")
	}
	return nil
}

func (o *LoginOptions) assertGatheredProjectInfo() error {
	if !o.gatherProjectInfo {
		return fmt.Errorf("Must gather project info first.")
	}
	return nil
}

func (o *LoginOptions) usernameProvided() bool {
	return len(o.Username) > 0
}

func (o *LoginOptions) passwordProvided() bool {
	return len(o.Password) > 0
}

func (o *LoginOptions) serverProvided() bool {
	_, err := o.ClientConfig.ClientConfig()
	return err == nil || !clientcmd.IsNoServerFound(err)
}

func usernameFromUser(user *api.User) string {
	return strings.Split(user.Name, ":")[1]
}
