package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"

	osclient "github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/api"
)

const WhoAmIRecommendedCommandName = "whoami"

const whoamiLong = `Show information about the current session

The default options for this command will return the currently authenticated user name
or an empty string.  Other flags support returning the currently used token or the
user context.
`

type WhoAmIOptions struct {
	UserInterface osclient.UserInterface

	Out io.Writer
}

func NewCmdWhoAmI(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &WhoAmIOptions{}

	cmd := &cobra.Command{
		Use:   name,
		Short: "Return information about the current session",
		Long:  whoamiLong,
		Run: func(cmd *cobra.Command, args []string) {
			err := RunWhoAmI(f, out, cmd, args, o)
			kcmdutil.CheckErr(err)
		},
	}
	cmd.Flags().BoolP("token", "t", false, "Print the token the current session is using. This will return an error if you are using a different form of authentication.")
	cmd.Flags().BoolP("context", "c", false, "Print the current user context name")

	return cmd
}

func (o WhoAmIOptions) WhoAmI() (*userapi.User, error) {
	me, err := o.UserInterface.Get("~")
	if err == nil {
		fmt.Fprintf(o.Out, "%s\n", me.Name)
	}

	return me, err
}

func RunWhoAmI(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string, o *WhoAmIOptions) error {
	if kcmdutil.GetFlagBool(cmd, "token") {
		cfg, err := f.OpenShiftClientConfig.ClientConfig()
		if err != nil {
			return err
		}
		if len(cfg.BearerToken) == 0 {
			return fmt.Errorf("no token is currently in use for this session")
		}
		fmt.Fprintf(out, "%s\n", cfg.BearerToken)
		return nil
	}
	if kcmdutil.GetFlagBool(cmd, "context") {
		cfg, err := f.OpenShiftClientConfig.RawConfig()
		if err != nil {
			return err
		}
		if len(cfg.CurrentContext) == 0 {
			return fmt.Errorf("no context has been set")
		}
		fmt.Fprintf(out, "%s\n", cfg.CurrentContext)
		return nil
	}

	client, _, err := f.Clients()
	if err != nil {
		return err
	}

	o.UserInterface = client.Users()
	o.Out = out

	_, err = o.WhoAmI()
	return err
}
