package admin

import (
	"errors"
	"fmt"
	"io"

	etcdclient "github.com/coreos/go-etcd/etcd"
	"github.com/spf13/cobra"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"
	kcmdutil "github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/cmd/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/tools"
	"github.com/openshift/origin/pkg/api/latest"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
	policyetcd "github.com/openshift/origin/pkg/authorization/registry/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/authorization/registry/policybinding/etcd"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"

	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicyetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	clusterpolicybindingetcd "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding/etcd"
	clusterroleregistry "github.com/openshift/origin/pkg/authorization/registry/clusterrole"
	clusterrolestorage "github.com/openshift/origin/pkg/authorization/registry/clusterrole/proxy"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding/proxy"

	"github.com/openshift/origin/pkg/cmd/cli/describe"
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
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
		Run: func(cmd *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}

			if err := options.OverwriteBootstrapPolicy(); err != nil {
				kcmdutil.CheckErr(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&options.Force, "force", "f", false, "You must confirm you really want to reset your policy. This will delete any custom settings you may have.")
	flags.StringVar(&options.File, "filename", "", "The policy template file containing roles and bindings.  One can be created with '"+createBootstrapPolicyCommand+"'.")
	flags.StringVar(&options.MasterConfigFile, "master-config", "origin.local.config/master/master-config.yaml", "Location of the master configuration file to run from in order to connect to etcd and directly modify the policy.")

	// autocompletion hints
	cmd.MarkFlagFilename("filename")
	cmd.MarkFlagFilename("master-config", "yaml", "yml")

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

	// Connect and setup etcd interfaces
	etcdClient, err := etcd.GetAndTestEtcdClient(masterConfig.EtcdClientInfo)
	if err != nil {
		return err
	}
	etcdHelper, err := newEtcdHelper(etcdClient, masterConfig.EtcdStorageConfig.OpenShiftStorageVersion, masterConfig.EtcdStorageConfig.OpenShiftStoragePrefix)
	if err != nil {
		return err
	}

	return OverwriteBootstrapPolicy(etcdHelper, o.File, o.CreateBootstrapPolicyCommand, o.Force, o.Out)
}

func OverwriteBootstrapPolicy(etcdHelper tools.EtcdHelper, policyFile, createBootstrapPolicyCommand string, change bool, out io.Writer) error {
	if !change {
		fmt.Fprintf(out, "Performing a dry run of policy overwrite:\n\n")
	}

	mapper := cmdclientcmd.ShortcutExpander{kubectl.ShortcutExpander{latest.RESTMapper}}
	typer := kapi.Scheme
	clientMapper := resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		return nil, nil
	})

	r := resource.NewBuilder(mapper, typer, clientMapper).
		FilenameParam(false, policyFile).
		Flatten().
		Do()

	if r.Err() != nil {
		return r.Err()
	}

	policyRegistry := policyregistry.NewRegistry(policyetcd.NewStorage(etcdHelper))
	policyBindingRegistry := policybindingregistry.NewRegistry(policybindingetcd.NewStorage(etcdHelper))

	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterpolicyetcd.NewStorage(etcdHelper))
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterpolicybindingetcd.NewStorage(etcdHelper))

	roleRegistry := roleregistry.NewRegistry(rolestorage.NewVirtualStorage(policyRegistry))
	roleBindingStorage := rolebindingstorage.NewVirtualStorage(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry)
	clusterRoleStorage := clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry)
	clusterRoleRegistry := clusterroleregistry.NewRegistry(clusterRoleStorage)
	clusterRoleBindingStorage := clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyRegistry, clusterPolicyBindingRegistry)

	return r.Visit(func(info *resource.Info) error {
		template, ok := info.Object.(*templateapi.Template)
		if !ok {
			return errors.New("policy must be contained in a template.  One can be created with '" + createBootstrapPolicyCommand + "'.")
		}
		runtime.DecodeList(template.Objects, kapi.Scheme)

		for _, item := range template.Objects {
			switch t := item.(type) {
			case *authorizationapi.Role:
				ctx := kapi.WithNamespace(kapi.NewContext(), t.Namespace)
				if change {
					roleRegistry.DeleteRole(ctx, t.Name)
					if _, err := roleRegistry.CreateRole(ctx, t); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRole(t); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}
			case *authorizationapi.RoleBinding:
				ctx := kapi.WithNamespace(kapi.NewContext(), t.Namespace)
				if change {
					roleBindingStorage.Delete(ctx, t.Name, nil)
					if _, err := roleBindingStorage.CreateRoleBindingWithEscalation(ctx, t); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role binding %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRoleBinding(t, nil, nil); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}

			case *authorizationapi.ClusterRole:
				ctx := kapi.WithNamespace(kapi.NewContext(), t.Namespace)
				if change {
					clusterRoleRegistry.DeleteClusterRole(ctx, t.Name)
					if _, err := clusterRoleRegistry.CreateClusterRole(ctx, t); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRole(authorizationapi.ToRole(t)); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}
			case *authorizationapi.ClusterRoleBinding:
				ctx := kapi.WithNamespace(kapi.NewContext(), t.Namespace)
				if change {
					clusterRoleBindingStorage.Delete(ctx, t.Name, nil)
					if _, err := clusterRoleBindingStorage.CreateClusterRoleBindingWithEscalation(ctx, t); err != nil {
						return err
					}
				} else {
					fmt.Fprintf(out, "Overwrite role binding %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRoleBinding(authorizationapi.ToRoleBinding(t), nil, nil); err == nil {
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

// newEtcdHelper returns an EtcdHelper for the provided storage version.
func newEtcdHelper(client *etcdclient.Client, version, prefix string) (oshelper tools.EtcdHelper, err error) {
	interfaces, err := latest.InterfacesFor(version)
	if err != nil {
		return tools.EtcdHelper{}, err
	}
	return tools.NewEtcdHelper(client, interfaces.Codec, prefix), nil
}
