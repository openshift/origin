package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
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
	Name     string
	FullName string

	UserClient userclient.UserInterface

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateUser is a macro command to create a new user
func NewCmdCreateUser(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateUserOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " USERNAME",
		Short:   "Manually create a user (only needed if automatic creation is disabled).",
		Long:    userLong,
		Example: fmt.Sprintf(userExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	cmd.Flags().StringVar(&o.FullName, "full-name", o.FullName, "Display name of the user")
	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *CreateUserOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("username is required")
	case 1:
		o.Name = args[0]
	default:
		return fmt.Errorf("exactly one argument (username) is supported, not: %v", args)
	}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	client, err := f.OpenshiftInternalUserClient()
	if err != nil {
		return err
	}

	o.UserClient = client.User()

	o.Mapper, _ = f.Object()
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, false, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateUserOptions) Validate() error {
	if len(o.Name) == 0 {
		return fmt.Errorf("username is required")
	}
	if o.UserClient == nil {
		return fmt.Errorf("UserClient is required")
	}
	if o.Mapper == nil {
		return fmt.Errorf("Mapper is required")
	}
	if o.Out == nil {
		return fmt.Errorf("Out is required")
	}
	if o.Printer == nil {
		return fmt.Errorf("Printer is required")
	}

	return nil
}

func (o *CreateUserOptions) Run() error {
	user := &userapi.User{}
	user.Name = o.Name
	user.FullName = o.FullName

	actualUser := user

	var err error
	if !o.DryRun {
		actualUser, err = o.UserClient.Users().Create(user)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "user", actualUser.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualUser, o.Out)
}
