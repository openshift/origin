package create

import (
	"fmt"

	"github.com/spf13/cobra"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/genericclioptions"

	userv1 "github.com/openshift/api/user/v1"
	userv1client "github.com/openshift/client-go/user/clientset/versioned/typed/user/v1"
)

const UserRecommendedName = "user"

var (
	userLong = templates.LongDesc(`
		This command can be used to create a user object.

		Typically, users are created automatically during login. If automatic
		creation is disabled (by using the "lookup" mapping method), users must
		be created manually.

		Corresponding identity and useridentitymapping objects must also be created
		to allow logging in as the created user.`)

	userExample = templates.Examples(`
		# Create a user with the username "ajones" and the display name "Adam Jones"
  	%[1]s ajones --full-name="Adam Jones"`)
)

type CreateUserOptions struct {
	CreateSubcommandOptions *CreateSubcommandOptions

	FullName string

	UserClient userv1client.UsersGetter
}

// NewCmdCreateUser is a macro command to create a new user
func NewCmdCreateUser(name, fullName string, f genericclioptions.RESTClientGetter, streams genericclioptions.IOStreams) *cobra.Command {
	o := &CreateUserOptions{
		CreateSubcommandOptions: NewCreateSubcommandOptions(streams),
	}
	cmd := &cobra.Command{
		Use:     name + " USERNAME",
		Short:   "Manually create a user (only needed if automatic creation is disabled).",
		Long:    userLong,
		Example: fmt.Sprintf(userExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.FullName, "full-name", o.FullName, "Display name of the user")

	o.CreateSubcommandOptions.PrintFlags.AddFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)

	return cmd
}

func (o *CreateUserOptions) Complete(cmd *cobra.Command, f genericclioptions.RESTClientGetter, args []string) error {
	clientConfig, err := f.ToRESTConfig()
	if err != nil {
		return err
	}
	o.UserClient, err = userv1client.NewForConfig(clientConfig)
	if err != nil {
		return err
	}

	return o.CreateSubcommandOptions.Complete(f, cmd, args)
}

func (o *CreateUserOptions) Run() error {
	user := &userv1.User{
		// this is ok because we know exactly how we want to be serialized
		TypeMeta: metav1.TypeMeta{APIVersion: userv1.SchemeGroupVersion.String(), Kind: "User"},
		ObjectMeta: metav1.ObjectMeta{
			Name: o.CreateSubcommandOptions.Name,
		},
		FullName: o.FullName,
	}
	var err error
	if !o.CreateSubcommandOptions.DryRun {
		user, err = o.UserClient.Users().Create(user)
		if err != nil {
			return err
		}
	}

	return o.CreateSubcommandOptions.Printer.PrintObj(user, o.CreateSubcommandOptions.Out)
}
