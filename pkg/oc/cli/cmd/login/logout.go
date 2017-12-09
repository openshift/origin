package login

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	restclient "k8s.io/client-go/rest"
	kclientcmd "k8s.io/client-go/tools/clientcmd"
	kclientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	oauthclient "github.com/openshift/origin/pkg/oauth/generated/internalclientset"
	"github.com/openshift/origin/pkg/oc/cli/config"
	osclientcmd "github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
)

type LogoutOptions struct {
	StartingKubeConfig *kclientcmdapi.Config
	Config             *restclient.Config
	Out                io.Writer

	PathOptions *kclientcmd.PathOptions
}

var (
	logoutLong = templates.LongDesc(`
		Log out of the active session out by clearing saved tokens

		An authentication token is stored in the config file after login - this command will delete
		that token on the server, and then remove the token from the configuration file.

		If you are using an alternative authentication method like Kerberos or client certificates,
		your ticket or client certificate will not be removed from the current system since these
		are typically managed by other programs. Instead, you can delete your config file to remove
		the local copy of that certificate or the record of your server login.

		After logging out, if you want to log back into the server use '%[1]s'.`)

	logoutExample = templates.Examples(`
	  # Logout
	  %[1]s`)
)

// NewCmdLogout implements the OpenShift cli logout command
func NewCmdLogout(name, fullName, ocLoginFullCommand string, f *osclientcmd.Factory, reader io.Reader, out io.Writer) *cobra.Command {
	options := &LogoutOptions{
		Out: out,
	}

	cmds := &cobra.Command{
		Use:     name,
		Short:   "End the current server session",
		Long:    fmt.Sprintf(logoutLong, ocLoginFullCommand),
		Example: fmt.Sprintf(logoutExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Complete(f, cmd, args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(err)
			}

			if err := options.RunLogout(); err != nil {
				kcmdutil.CheckErr(err)
			}

		},
	}

	// TODO: support --all which performs the same logic on all users in your config file.

	return cmds
}

func (o *LogoutOptions) Complete(f *osclientcmd.Factory, cmd *cobra.Command, args []string) error {
	kubeconfig, err := f.OpenShiftClientConfig().RawConfig()
	o.StartingKubeConfig = &kubeconfig
	if err != nil {
		return err
	}

	o.Config, err = f.OpenShiftClientConfig().ClientConfig()
	if err != nil {
		return err
	}

	o.PathOptions = config.NewPathOptions(cmd)

	return nil
}

func (o LogoutOptions) Validate(args []string) error {
	if len(args) > 0 {
		return errors.New("No arguments are allowed")
	}

	if o.StartingKubeConfig == nil {
		return errors.New("Must have a config file already created")
	}

	if len(o.Config.BearerToken) == 0 {
		return errors.New("You must have a token in order to logout.")
	}

	return nil
}

func (o LogoutOptions) RunLogout() error {
	token := o.Config.BearerToken

	client, err := oauthclient.NewForConfig(o.Config)
	if err != nil {
		return err
	}

	userInfo, err := whoAmI(o.Config)
	if err != nil {
		return err
	}

	if err := client.Oauth().OAuthAccessTokens().Delete(token, &metav1.DeleteOptions{}); err != nil {
		glog.V(1).Infof("%v", err)
	}

	configErr := deleteTokenFromConfig(*o.StartingKubeConfig, o.PathOptions, token)
	if configErr == nil {
		glog.V(1).Infof("Removed token from your local configuration.")

		// only return error instead of successful message if removing token from client
		// config fails. Any error that occurs deleting token using api is logged above.
		fmt.Fprintf(o.Out, "Logged %q out on %q\n", userInfo.Name, o.Config.Host)
	}

	return configErr
}

func deleteTokenFromConfig(config kclientcmdapi.Config, pathOptions *kclientcmd.PathOptions, bearerToken string) error {
	for key, value := range config.AuthInfos {
		if value.Token == bearerToken {
			value.Token = ""
			config.AuthInfos[key] = value
			// don't break, its possible that more than one user stanza has the same token.
		}
	}

	return kclientcmd.ModifyConfig(pathOptions, config, true)
}
