package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	cmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/templates"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const longDescription = `Logs in to the OpenShift server and saves a config file that
will be used by subsequent commands.

First-time users of the OpenShift client must run this command to configure the server,
establish a session against it, and save it to a configuration file. The default
configuration will be in your home directory under ".config/openshift/config".

The information required to login, like username and password, a session token, or
the server details, can be provided through flags. If not provided, the command will
prompt for user input as needed.
`

// NewCmdLogin implements the OpenShift cli login command
func NewCmdLogin(f *osclientcmd.Factory, reader io.Reader, out io.Writer) *cobra.Command {
	options := &LoginOptions{
		Reader:       reader,
		Out:          out,
		ClientConfig: f.OpenShiftClientConfig,
	}

	cmds := &cobra.Command{
		Use:   "login [--username=<username>] [--password=<password>] [--server=<server>] [--context=<context>] [--certificate-authority=<path>]",
		Short: "Logs in and save the configuration",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunLogin(cmd, options)

			if errors.IsUnauthorized(err) {
				fmt.Fprintln(out, "Login failed (401 Unauthorized)")

				if err, isStatusErr := err.(*errors.StatusError); isStatusErr {
					if details := err.Status().Details; details != nil {
						for _, cause := range details.Causes {
							fmt.Fprintln(out, cause.Message)
						}
					}
				}

				os.Exit(1)

			} else {
				cmdutil.CheckErr(err)
			}
		},
	}

	// Login is the only command that can negotiate a session token against the auth server using basic auth
	cmds.Flags().StringVarP(&options.Username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.Password, "password", "p", "", "Password, will prompt if not provided")

	templater := templates.Templater{
		UsageTemplate: templates.MainUsageTemplate(),
		Exposed:       []string{"server", "certificate-authority", "insecure-skip-tls-verify", "context"},
	}
	cmds.SetUsageFunc(templater.UsageFunc())
	cmds.SetHelpTemplate(templates.MainHelpTemplate())

	return cmds
}

// RunLogin contains all the necessary functionality for the OpenShift cli login command
func RunLogin(cmd *cobra.Command, options *LoginOptions) error {
	if certFile := cmdutil.GetFlagString(cmd, "client-certificate"); len(certFile) > 0 {
		options.CertFile = certFile
	}
	if keyFile := cmdutil.GetFlagString(cmd, "client-key"); len(keyFile) > 0 {
		options.KeyFile = keyFile
	}
	options.PathToSaveConfig = cmdutil.GetFlagString(cmd, config.OpenShiftConfigFlagName)

	if err := options.GatherInfo(); err != nil {
		return err
	}

	newFileCreated, err := options.SaveConfig()
	if err != nil {
		return err
	}

	if newFileCreated {
		fmt.Fprintln(options.Out, "Welcome to OpenShift! See 'osc help' to get started.")
	}
	return nil
}
