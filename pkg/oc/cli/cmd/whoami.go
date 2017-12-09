package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

const WhoAmIRecommendedCommandName = "whoami"

var whoamiLong = templates.LongDesc(`
	Show information about the current session

	The default options for this command will return the currently authenticated user name
	or an empty string.  Other flags support returning the currently used token or the
	user context.`)

type WhoAmIOptions struct {
	UserInterface userclient.UserResourceInterface

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

	cmd.Flags().BoolP("show-token", "t", false, "Print the token the current session is using. This will return an error if you are using a different form of authentication.")
	cmd.Flags().BoolP("show-context", "c", false, "Print the current user context name")
	cmd.Flags().Bool("show-server", false, "If true, print the current server's REST API URL")

	return cmd
}

func (o WhoAmIOptions) WhoAmI() (*userapi.User, error) {
	me, err := o.UserInterface.Get("~", metav1.GetOptions{})
	if err == nil {
		fmt.Fprintf(o.Out, "%s\n", me.Name)
	}

	return me, err
}

func RunWhoAmI(f *clientcmd.Factory, out io.Writer, cmd *cobra.Command, args []string, o *WhoAmIOptions) error {
	if kcmdutil.GetFlagBool(cmd, "show-token") {
		cfg, err := f.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return err
		}
		if len(cfg.BearerToken) == 0 {
			return fmt.Errorf("no token is currently in use for this session")
		}
		fmt.Fprintf(out, "%s\n", cfg.BearerToken)
		return nil
	}
	if kcmdutil.GetFlagBool(cmd, "show-context") {
		cfg, err := f.OpenShiftClientConfig().RawConfig()
		if err != nil {
			return err
		}
		if len(cfg.CurrentContext) == 0 {
			return fmt.Errorf("no context has been set")
		}
		fmt.Fprintf(out, "%s\n", cfg.CurrentContext)
		return nil
	}
	if kcmdutil.GetFlagBool(cmd, "show-server") {
		cfg, err := f.OpenShiftClientConfig().ClientConfig()
		if err != nil {
			return err
		}
		fmt.Fprintf(out, "%s\n", cfg.Host)
		return nil
	}

	client, err := f.OpenshiftInternalUserClient()
	if err != nil {
		return err
	}

	o.UserInterface = client.User().Users()
	o.Out = out

	_, err = o.WhoAmI()
	return err
}
