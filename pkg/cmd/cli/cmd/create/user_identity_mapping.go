package create

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/meta"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/cmd/templates"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/api"
)

const UserIdentityMappingRecommendedName = "useridentitymapping"

var (
	userIdentityMappingLong = templates.LongDesc(`
		Typically, identities are automatically mapped to users during login. If automatic
		mapping is disabled (by using the "lookup" mapping method), or a mapping needs to
		be manually established between an identity and a user, this command can be used
		to create a useridentitymapping object.`)

	userIdentityMappingExample = templates.Examples(`
		# Map the identity "acme_ldap:adamjones" to the user "ajones"
  	%[1]s acme_ldap:adamjones ajones`)
)

type CreateUserIdentityMappingOptions struct {
	User     string
	Identity string

	UserIdentityMappingClient client.UserIdentityMappingInterface

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateUserIdentityMapping is a macro command to create a new identity
func NewCmdCreateUserIdentityMapping(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateUserIdentityMappingOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " <IDENTITY_NAME> <USER_NAME>",
		Short:   "Manually map an identity to a user.",
		Long:    userIdentityMappingLong,
		Example: fmt.Sprintf(userIdentityMappingExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}
	cmdutil.AddPrinterFlags(cmd)
	cmdutil.AddDryRunFlag(cmd)
	return cmd
}

func (o *CreateUserIdentityMappingOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("identity is required")
	case 1:
		return fmt.Errorf("user name is required")
	case 2:
		o.Identity = args[0]
		o.User = args[1]
	default:
		return fmt.Errorf("exactly two arguments (identity and user name) are supported, not: %v", args)
	}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	client, _, err := f.Clients()
	if err != nil {
		return err
	}
	o.UserIdentityMappingClient = client.UserIdentityMappings()

	o.Mapper, _ = f.Object(false)
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateUserIdentityMappingOptions) Validate() error {
	if len(o.Identity) == 0 {
		return fmt.Errorf("identity is required")
	}
	if len(o.User) == 0 {
		return fmt.Errorf("user is required")
	}
	if o.UserIdentityMappingClient == nil {
		return fmt.Errorf("UserIdentityMappingClient is required")
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

func (o *CreateUserIdentityMappingOptions) Run() error {
	mapping := &userapi.UserIdentityMapping{}
	mapping.Identity = kapi.ObjectReference{Name: o.Identity}
	mapping.User = kapi.ObjectReference{Name: o.User}

	actualMapping := mapping

	var err error
	if !o.DryRun {
		actualMapping, err = o.UserIdentityMappingClient.Create(mapping)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "useridentitymapping", actualMapping.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualMapping, o.Out)
}
