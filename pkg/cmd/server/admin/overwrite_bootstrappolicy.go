package admin

import (
	"errors"
	"fmt"

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
	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	configvalidation "github.com/openshift/origin/pkg/cmd/server/api/validation"
	"github.com/openshift/origin/pkg/cmd/server/etcd"
	cmdclientcmd "github.com/openshift/origin/pkg/cmd/util/clientcmd"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

type OverwriteBootstrapPolicyOptions struct {
	File             string
	MasterConfigFile string
}

func NewCommandOverwriteBootstrapPolicy() *cobra.Command {
	options := &OverwriteBootstrapPolicyOptions{}

	cmd := &cobra.Command{
		Use:   "overwrite-policy",
		Short: "Overwrite policy for OpenShift.  DANGER: THIS BYPASSES ALL ACCESS CONTROL CHECKS AND WRITES DIRECTLY TO ETCD!",
		Run: func(c *cobra.Command, args []string) {
			if err := options.Validate(args); err != nil {
				fmt.Println(err.Error())
				c.Help()
				return
			}

			if err := options.OverwriteBootstrapPolicy(); err != nil {
				glog.Fatal(err)
			}
		},
	}

	flags := cmd.Flags()

	flags.StringVar(&options.File, "filename", "", "The policy template file containing roles and bindings.  One can be created with '"+CreateBootstrapPolicyFileFullCommand+"'.")
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

	etcdHelper, err := etcd.NewOpenShiftEtcdHelper(masterConfig.EtcdClientInfo.URL)
	if err != nil {
		return err
	}

	return OverwriteBootstrapPolicy(etcdHelper, masterConfig.PolicyConfig.MasterAuthorizationNamespace, o.File)
}

func OverwriteBootstrapPolicy(etcdHelper tools.EtcdHelper, masterNamespace, policyFile string) error {
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
			return errors.New("policy must be contained in a template.  One can be created with '" + CreateBootstrapPolicyFileFullCommand + "'.")
		}

		for _, item := range template.Objects {
			switch castObject := item.(type) {
			case *authorizationapi.Role:
				ctx := api.WithNamespace(api.NewContext(), castObject.Namespace)
				roleRegistry.DeleteRole(ctx, castObject.Name)
				if err := roleRegistry.CreateRole(ctx, castObject); err != nil {
					return err
				}

			case *authorizationapi.RoleBinding:
				ctx := api.WithNamespace(api.NewContext(), castObject.Namespace)
				roleBindingRegistry.DeleteRoleBinding(ctx, castObject.Name)
				if err := roleBindingRegistry.CreateRoleBinding(ctx, castObject, true); err != nil {
					return err
				}

			default:
				return errors.New("only roles and rolebindings may be created in this mode")
			}
		}

		return nil
	})
}
