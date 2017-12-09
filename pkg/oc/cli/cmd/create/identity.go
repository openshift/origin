package create

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/kubernetes/pkg/kubectl/cmd/templates"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/openshift/origin/pkg/oc/cli/util/clientcmd"
	userapi "github.com/openshift/origin/pkg/user/apis/user"
	userclient "github.com/openshift/origin/pkg/user/generated/internalclientset/typed/user/internalversion"
)

const IdentityRecommendedName = "identity"

var (
	identityLong = templates.LongDesc(`
		This command can be used to create an identity object.

		Typically, identities are created automatically during login. If automatic
		creation is disabled (by using the "lookup" mapping method), identities must
		be created manually.

		Corresponding user and useridentitymapping objects must also be created
		to allow logging in with the created identity.`)

	identityExample = templates.Examples(`
		# Create an identity with identity provider "acme_ldap" and the identity provider username "adamjones"
  	%[1]s acme_ldap:adamjones`)
)

type CreateIdentityOptions struct {
	ProviderName     string
	ProviderUserName string

	IdentityClient userclient.IdentityInterface

	DryRun bool

	Mapper       meta.RESTMapper
	OutputFormat string
	Out          io.Writer
	Printer      ObjectPrinter
}

// NewCmdCreateIdentity is a macro command to create a new identity
func NewCmdCreateIdentity(name, fullName string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	o := &CreateIdentityOptions{Out: out}

	cmd := &cobra.Command{
		Use:     name + " <PROVIDER_NAME>:<PROVIDER_USER_NAME>",
		Short:   "Manually create an identity (only needed if automatic creation is disabled).",
		Long:    identityLong,
		Example: fmt.Sprintf(identityExample, fullName),
		Run: func(cmd *cobra.Command, args []string) {
			cmdutil.CheckErr(o.Complete(cmd, f, args))
			cmdutil.CheckErr(o.Validate())
			cmdutil.CheckErr(o.Run())
		},
	}

	cmdutil.AddDryRunFlag(cmd)
	cmdutil.AddPrinterFlags(cmd)
	return cmd
}

func (o *CreateIdentityOptions) Complete(cmd *cobra.Command, f *clientcmd.Factory, args []string) error {
	switch len(args) {
	case 0:
		return fmt.Errorf("identity name in the format <PROVIDER_NAME>:<PROVIDER_USER_NAME> is required")
	case 1:
		parts := strings.Split(args[0], ":")
		if len(parts) != 2 {
			return fmt.Errorf("identity name in the format <PROVIDER_NAME>:<PROVIDER_USER_NAME> is required")
		}
		o.ProviderName = parts[0]
		o.ProviderUserName = parts[1]
	default:
		return fmt.Errorf("exactly one argument (username) is supported, not: %v", args)
	}

	o.DryRun = cmdutil.GetFlagBool(cmd, "dry-run")

	client, err := f.OpenshiftInternalUserClient()
	if err != nil {
		return err
	}
	o.IdentityClient = client.User().Identities()

	o.Mapper, _ = f.Object()
	o.OutputFormat = cmdutil.GetFlagString(cmd, "output")

	o.Printer = func(obj runtime.Object, out io.Writer) error {
		return f.PrintObject(cmd, false, o.Mapper, obj, out)
	}

	return nil
}

func (o *CreateIdentityOptions) Validate() error {
	if len(o.ProviderName) == 0 {
		return fmt.Errorf("provider name is required")
	}
	if len(o.ProviderUserName) == 0 {
		return fmt.Errorf("provider user name is required")
	}
	if o.IdentityClient == nil {
		return fmt.Errorf("IdentityClient is required")
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

func (o *CreateIdentityOptions) Run() error {
	identity := &userapi.Identity{}
	identity.ProviderName = o.ProviderName
	identity.ProviderUserName = o.ProviderUserName

	actualIdentity := identity

	var err error
	if !o.DryRun {
		actualIdentity, err = o.IdentityClient.Create(identity)
		if err != nil {
			return err
		}
	}

	if useShortOutput := o.OutputFormat == "name"; useShortOutput || len(o.OutputFormat) == 0 {
		cmdutil.PrintSuccess(o.Mapper, useShortOutput, o.Out, "identity", actualIdentity.Name, o.DryRun, "created")
		return nil
	}

	return o.Printer(actualIdentity, o.Out)
}
