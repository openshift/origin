package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/openshift/origin/pkg/cmd/cli/config"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const longDescription = `Logs in to the OpenShift server and save the session
information to a config file that will be used by every subsequent command.

First-time users of the OpenShift client must run this command to configure the server,
establish a session against it and save it to a configuration file, usually in the
user's home directory.

The information required to login, like username and password or a session token, and 
the server details, can be provided through flags. If not provided, the command will
prompt for user input if needed.
`

func NewCmdLogin(f *osclientcmd.Factory, reader io.Reader, out io.Writer) *cobra.Command {
	options := &LoginOptions{}

	cmds := &cobra.Command{
		Use:   "login [--username=<username>] [--password=<password>] [--server=<server>] [--context=<context>] [--certificate-authority=<path>]",
		Short: "Logs in and save the configuration",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			options.Reader = reader
			options.ClientConfig = f.OpenShiftClientConfig

			checkErr(options.GatherInfo())

			forcePath := cmdutil.GetFlagString(cmd, config.OpenShiftConfigFlagName)
			options.PathToSaveConfig = forcePath

			newFileCreated, err := options.SaveConfig()
			checkErr(err)

			if newFileCreated {
				fmt.Println("Welcome to OpenShift v3! Use 'osc --help' for a list of commands available.")
			}
		},
	}

	// TODO flags below should be DE-REGISTERED from the persistent flags and kept only here.
	// Login is the only command that can negotiate a session token against the auth server.
	cmds.Flags().StringVarP(&options.Username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.Password, "password", "p", "", "Password, will prompt if not provided")
	return cmds
}
