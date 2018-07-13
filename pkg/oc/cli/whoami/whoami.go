package whoami

import (
	"fmt"

	"github.com/spf13/cobra"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd/api"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclientinternal "github.com/openshift/origin/pkg/user/generated/internalclientset"
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

	ClientConfig *rest.Config
	RawConfig    api.Config

	ShowToken   bool
	ShowContext bool
	ShowServer  bool

	genericclioptions.IOStreams
}

func NewWhoAmIOptions(streams genericclioptions.IOStreams) *WhoAmIOptions {
	return &WhoAmIOptions{
		IOStreams: streams,
	}
}

func NewCmdWhoAmI(name, fullName string, f kcmdutil.Factory, streams genericclioptions.IOStreams) *cobra.Command {
	o := NewWhoAmIOptions(streams)

	cmd := &cobra.Command{
		Use:   name,
		Short: "Return information about the current session",
		Long:  whoamiLong,
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(o.Complete(f))
			kcmdutil.CheckErr(o.Validate())
			kcmdutil.CheckErr(o.Run())
		},
	}

	cmd.Flags().BoolVarP(&o.ShowToken, "show-token", "t", o.ShowToken, "Print the token the current session is using. This will return an error if you are using a different form of authentication.")
	cmd.Flags().BoolVarP(&o.ShowContext, "show-context", "c", o.ShowContext, "Print the current user context name")
	cmd.Flags().BoolVar(&o.ShowServer, "show-server", o.ShowServer, "If true, print the current server's REST API URL")

	return cmd
}

func (o WhoAmIOptions) WhoAmI() (*userapi.User, error) {
	me, err := o.UserInterface.Get("~", metav1.GetOptions{})
	if err == nil {
		fmt.Fprintf(o.Out, "%s\n", me.Name)
	}

	return me, err
}

func (o *WhoAmIOptions) Complete(f kcmdutil.Factory) error {
	var err error
	o.ClientConfig, err = f.ToRESTConfig()
	if err != nil {
		return err
	}

	o.RawConfig, err = f.ToRawKubeConfigLoader().RawConfig()
	return err
}

func (o *WhoAmIOptions) Validate() error {
	if o.ShowToken && len(o.ClientConfig.BearerToken) == 0 {
		return fmt.Errorf("no token is currently in use for this session")
	}
	if o.ShowContext && len(o.RawConfig.CurrentContext) == 0 {
		return fmt.Errorf("no context has been set")
	}

	return nil
}

func (o *WhoAmIOptions) Run() error {
	if o.ShowToken {
		fmt.Fprintf(o.Out, "%s\n", o.ClientConfig.BearerToken)
		return nil
	}
	if o.ShowContext {
		fmt.Fprintf(o.Out, "%s\n", o.RawConfig.CurrentContext)
		return nil
	}
	if o.ShowServer {
		fmt.Fprintf(o.Out, "%s\n", o.ClientConfig.Host)
		return nil
	}

	client, err := userclientinternal.NewForConfig(o.ClientConfig)
	if err != nil {
		return err
	}

	o.UserInterface = client.User().Users()

	_, err = o.WhoAmI()
	return err
}
