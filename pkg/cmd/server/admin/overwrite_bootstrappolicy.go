package admin

import (
	"errors"
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	utilerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	authorizationetcd "github.com/openshift/origin/pkg/authorization/registry/etcd"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	"github.com/openshift/origin/pkg/cmd/cli/describe"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

const OverwriteBootstrapPolicyCommandName = "overwrite-policy"

type OverwriteBootstrapPolicyOptions struct {
	File             string
	MasterConfigFile string

	Force                        bool
	Out                          io.Writer
	CreateBootstrapPolicyCommand string
}

func NewCommandOverwriteBootstrapPolicy(commandName string, fullName string, createBootstrapPolicyCommand string, out io.Writer) *cobra.Command {
	options := &OverwriteBootstrapPolicyOptions{Out: out}
	options.CreateBootstrapPolicyCommand = createBootstrapPolicyCommand

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Reset the policy to the default values",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Fprintln(c.Out(), err.Error())
				c.Help()
				return
			}

			if err := options.OverwriteBootstrapPolicy(); err != nil {
				glog.Fatal(err)
			}
		},
	}
	cmd.SetOutput(out)

	flags := cmd.Flags()

	flags.BoolVarP(&options.Force, "force", "f", false, "You must confirm you really want to reset your policy. This will delete any custom settings you may have.")
	flags.StringVar(&options.File, "filename", "", "The policy template file containing roles and bindings.  One can be created with '"+createBootstrapPolicyCommand+"'.")
	flags.StringVar(&options.MasterConfigFile, "master-config", "master.yaml", "Location of the master configuration file to run from in order to connect to etcd and directly modify the policy.")

	return cmd
}

func (o OverwriteBootstrapPolicyOptions) Validate(args []string) error {
	if len(args) != 0 {
		return errors.New("no arguments are supported")
	}
	if len(o.File) == 0 {
		return errors.New("filename must be provided")
	}
	if len(o.MasterConfigFile) == 0 {
		return errors.New("master-config must be provided")
	}

	return nil
}

func (o OverwriteBootstrapPolicyOptions) OverwriteBootstrapPolicy() error {
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.MasterConfigFile)
	if err != nil {
		return err
	}
	if err := configvalidation.ValidateNamespace(masterConfig.PolicyConfig.MasterAuthorizationNamespace, "masterAuthorizationNamespace"); len(err) > 0 {
		return utilerrors.NewAggregate(err)
	}

	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(masterConfig.EtcdClientInfo)
	if err != nil {
		return err
	}

	return OverwriteBootstrapPolicy(etcdHelper, masterConfig.PolicyConfig.MasterAuthorizationNamespace, o.File, o.CreateBootstrapPolicyCommand, o.Force, o.Out)
}

func OverwriteBootstrapPolicy(etcdHelper tools.EtcdHelper, masterNamespace, policyFile, createBootstrapPolicyCommand string, change bool, out io.Writer) error {
	if !change {
		fmt.Fprintf(out, "Performing a dry run of policy overwrite:\n\n")
	}

	mapper := cmdclientcmd.ShortcutExpander{kubectl.ShortcutExpander{latest.RESTMapper}}
	typer := api.Scheme
	clientMapper := resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		return nil, nil
	})

	r := resource.NewBuilder(mapper, typer, clientMapper).
		FilenameParam(policyFile).
		Flatten().
		Do()

	if r.Err() != nil {
		return r.Err()
	}

	registry := authorizationetcd.New(etcdHelper)
	roleRegistry := roleregistry.NewVirtualRegistry(registry)
	roleBindingRegistry := rolebindingregistry.NewVirtualRegistry(registry, registry, masterNamespace)

	return r.Visit(func(info *resource.Info) error {
		template, ok := info.Object.(*templateapi.Template)
		if !ok {
			return errors.New("policy must be contained in a template.  One can be created with '" + createBootstrapPolicyCommand + "'.")
		}

		for _, item := range template.Objects {
			switch t := item.(type) {
			case *authorizationapi.Role:
				ctx := api.WithNamespace(api.NewContext(), t.Namespace)
				if change {
					roleRegistry.DeleteRole(ctx, t.Name)
					if err := roleRegistry.CreateRole(ctx, t); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRole(t); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}
			case *authorizationapi.RoleBinding:
				ctx := api.WithNamespace(api.NewContext(), t.Namespace)
				if change {
					roleBindingRegistry.DeleteRoleBinding(ctx, t.Name)
					if err := roleBindingRegistry.CreateRoleBinding(ctx, t, true); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role binding %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRoleBinding(t, nil, nil); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}

			default:
				return errors.New("only roles and rolebindings may be created in this mode")
			}
		}
		if !change {
			fmt.Fprintf(out, "To make the changes described above, pass --force\n")
		}
		return nil
	})
}
