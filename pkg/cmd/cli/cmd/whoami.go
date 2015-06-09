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

type WhoAmIOptions struct {
	UserInterface osclient.UserInterface

	Out io.Writer
}

func NewCmdWhoAmI(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &WhoAmIOptions{}

	cmd := &cobra.Command{
		Use:   name,
		Short: "displays the username of the currently authenticated user",
		Long:  `displays the username of the currently authenticated user`,
		Run: func(cmd *cobra.Command, args []string) {
			client, _, err := f.Clients()
			kcmdutil.CheckErr(err)

			o.UserInterface = client.Users()
			o.Out = out

			_, err = o.WhoAmI()
			kcmdutil.CheckErr(err)
		},
	}
	return cmd
}

func (o WhoAmIOptions) WhoAmI() (*userapi.User, error) {
	me, err := o.UserInterface.Get("~")
	if err == nil {
		fmt.Fprintf(o.Out, "%s\n", me.Name)
	}

	return me, err
}
