package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kcmdconfig "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/config"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

type LogoutOptions struct {
	StartingKubeConfig *kclientcmdapi.Config
	Config             *kclient.Config
	Out                io.Writer

	PathOptions *kcmdconfig.PathOptions
}

const (
	logoutLong = `
Log out of the active session out by clearing saved tokens

An authentication token is stored in the config file after login - this command will delete
that token on the server, and then remove the token from the configuration file.

If you are using an alternative authentication method like Kerberos or client certificates,
your ticket or client certificate will not be removed from the current system since these
are typically managed by other programs. Instead, you can delete your config file to remove
the local copy of that certificate or the record of your server login.

After logging out, if you want to log back into the OpenShift server, try '%[1]s'.`

	logoutExample = `  // Logout
  $ %[1]s`
)

// NewCmdLogout implements the OpenShift cli logout command
func NewCmdLogout(name, fullName, oscLoginFullCommand string, f *osclientcmd.Factory, reader io.Reader, out io.Writer) *cobra.Command {
	options := &LogoutOptions{
		Out: out,
	}

	cmds := &cobra.Command{
		Use:     name,
		Short:   "End the current server session",
		Long:    fmt.Sprintf(logoutLong, oscLoginFullCommand),
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
	kubeconfig, err := f.OpenShiftClientConfig.RawConfig()
	o.StartingKubeConfig = &kubeconfig
	if err != nil {
		return err
	}

	o.Config, err = f.OpenShiftClientConfig.ClientConfig()
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

	client, err := client.New(o.Config)
	if err != nil {
		return err
	}

	userInfo, err := whoAmI(client)
	if err != nil {
		return err
	}

	if err := client.OAuthAccessTokens().Delete(token); err != nil {
		return err
	}

	newConfig := *o.StartingKubeConfig

	for key, value := range newConfig.AuthInfos {
		if value.Token == token {
			value.Token = ""
			newConfig.AuthInfos[key] = value
			// don't break, its possible that more than one user stanza has the same token.
		}
	}

	if err := kcmdconfig.ModifyConfig(o.PathOptions, newConfig); err != nil {
		return err
	}

	fmt.Fprintf(o.Out, "Logged %q out on %q\n", userInfo.Name, o.Config.Host)

	return nil
}
