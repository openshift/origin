package cmd

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"

	kapierrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client"
	kclientcmdapi "github.com/GoogleCloudPlatform/kubernetes/pkg/client/clientcmd/api"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/cmd/cli/config"
	"github.com/openshift/origin/pkg/cmd/flagtypes"
	"github.com/openshift/origin/pkg/cmd/templates"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	osclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

// Helper for the login and setup process, gathers all information required for a
// successful login and eventual update of config files.
// Depending on the Reader present it can be interactive, asking for terminal input in
// case of any missing information.
// Notice that some methods mutate this object so it should not be reused. The Config
// provided as a pointer will also mutate (handle new auth tokens, etc).
type LoginOptions struct {
	Server string

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
	CAFile      string
	CertFile    string
	KeyFile     string
	InsecureTLS bool

	// Optional, if provided will only try to save in it
	PathToSaveConfig string
}

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
		Reader: reader,
		Out:    out,
	}

	cmds := &cobra.Command{
		Use:   "login [server URL] [--username=<username>] [--password=<password>] [--certificate-authority=<path>]",
		Short: "Logs in and save the configuration",
		Long:  longDescription,
		Run: func(cmd *cobra.Command, args []string) {
			options.Complete(f, cmd, args)

			if err := options.Validate(args, kcmdutil.GetFlagString(cmd, "server")); err != nil {
				kcmdutil.CheckErr(err)
			}

			err := RunLogin(cmd, options)

			if kapierrors.IsUnauthorized(err) {
				fmt.Fprintln(out, "Login failed (401 Unauthorized)")

				if err, isStatusErr := err.(*kapierrors.StatusError); isStatusErr {
					if details := err.Status().Details; details != nil {
						for _, cause := range details.Causes {
							fmt.Fprintln(out, cause.Message)
						}
					}
				}

				os.Exit(1)

			} else {
				kcmdutil.CheckErr(err)
			}
		},
	}

	// Login is the only command that can negotiate a session token against the auth server using basic auth
	cmds.Flags().StringVarP(&options.Username, "username", "u", "", "Username, will prompt if not provided")
	cmds.Flags().StringVarP(&options.Password, "password", "p", "", "Password, will prompt if not provided")

	templater := templates.Templater{
		UsageTemplate: templates.MainUsageTemplate(),
		Exposed:       []string{"certificate-authority", "insecure-skip-tls-verify"},
	}
	cmds.SetUsageFunc(templater.UsageFunc())
	cmds.SetHelpTemplate(templates.MainHelpTemplate())

	return cmds
}

func (o *LoginOptions) Complete(f *osclientcmd.Factory, cmd *cobra.Command, args []string) error {
	kubeconfig, err := f.OpenShiftClientConfig.RawConfig()
	if err != nil {
		return err
	}
	o.StartingKubeConfig = &kubeconfig

	if serverFlag := kcmdutil.GetFlagString(cmd, "server"); len(serverFlag) > 0 {
		o.Server = serverFlag

	} else if len(args) == 1 {
		addr := flagtypes.Addr{Value: "localhost:8443", DefaultScheme: "https", DefaultPort: 8443, AllowPrefix: true}.Default()
		if err := addr.Set(args[0]); err != nil {
			return err
		}
		o.Server = addr.String()

	} else if len(o.Server) == 0 {
		if defaultContext, defaultContextExists := o.StartingKubeConfig.Contexts[o.StartingKubeConfig.CurrentContext]; defaultContextExists {
			if cluster, exists := o.StartingKubeConfig.Clusters[defaultContext.Cluster]; exists {
				o.Server = cluster.Server
			}
		}

	}

	if certFile := kcmdutil.GetFlagString(cmd, "client-certificate"); len(certFile) > 0 {
		o.CertFile = certFile
	}
	if keyFile := kcmdutil.GetFlagString(cmd, "client-key"); len(keyFile) > 0 {
		o.KeyFile = keyFile
	}
	o.PathToSaveConfig = kcmdutil.GetFlagString(cmd, config.OpenShiftConfigFlagName)

	o.CAFile = kcmdutil.GetFlagString(cmd, "certificate-authority")
	o.InsecureTLS = kcmdutil.GetFlagBool(cmd, "insecure-skip-tls-verify")

	o.DefaultNamespace, _ = f.OpenShiftClientConfig.Namespace()

	return nil
}

func (o LoginOptions) Validate(args []string, serverFlag string) error {
	if len(args) > 1 {
		return errors.New("Only the server URL may be specified as an argument")
	}

	if (len(serverFlag) > 0) && (len(args) == 1) {
		return errors.New("--server and passing the server URL as an argument are mutually exclusive")
	}

	if (len(o.Server) == 0) && !cmdutil.IsTerminal(o.Reader) {
		return errors.New("A server URL must be specified")
	}

	return nil
}

// RunLogin contains all the necessary functionality for the OpenShift cli login command
func RunLogin(cmd *cobra.Command, options *LoginOptions) error {
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
