package admin

import (
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/spf13/cobra"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime"
	kerrors "k8s.io/apimachinery/pkg/util/errors"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"
	kcmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"

	configapilatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	originrest "github.com/openshift/origin/pkg/cmd/server/origin/rest"
	"github.com/openshift/origin/pkg/oc/cli/describe"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"github.com/openshift/origin/pkg/util/restoptions"

	"github.com/openshift/origin/pkg/authorization/rulevalidation"
	clusterpolicyregistry "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterpolicy"
	clusterpolicyetcd "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterpolicy/etcd"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterpolicybinding"
	clusterpolicybindingetcd "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterpolicybinding/etcd"
	"github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterrole"
	clusterrolestorage "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterrole/proxy"
	"github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterrolebinding"
	clusterrolebindingstorage "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/clusterrolebinding/proxy"
	policyregistry "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/policy"
	policyetcd "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/policy/etcd"
	policybindingregistry "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/policybinding"
	policybindingetcd "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/policybinding/etcd"
	"github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/role"
	rolestorage "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/role/policybased"
	"github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/rolebinding"
	rolebindingstorage "github.com/openshift/origin/pkg/cmd/server/admin/legacyetcd/rolebinding/policybased"
	"github.com/openshift/origin/pkg/cmd/util/clientcmd"
)

const OverwriteBootstrapPolicyCommandName = "overwrite-policy"

type OverwriteBootstrapPolicyOptions struct {
	File             string
	MasterConfigFile string

	Force                        bool
	Out                          io.Writer
	CreateBootstrapPolicyCommand string
}

func NewCommandOverwriteBootstrapPolicy(commandName string, fullName string, createBootstrapPolicyCommand string, f *clientcmd.Factory, out io.Writer) *cobra.Command {
	options := &OverwriteBootstrapPolicyOptions{Out: out}
	options.CreateBootstrapPolicyCommand = createBootstrapPolicyCommand

	cmd := &cobra.Command{
		Use:   commandName,
		Short: "Reset the policy to the default values",
		Run: func(cmd *cobra.Command, args []string) {
			kcmdutil.CheckErr(options.Complete(f))
			if err := options.Validate(args); err != nil {
				kcmdutil.CheckErr(kcmdutil.UsageError(cmd, err.Error()))
			}
			kcmdutil.CheckErr(options.OverwriteBootstrapPolicy())
		},
	}

	flags := cmd.Flags()

	flags.BoolVarP(&options.Force, "force", "f", false, "You must confirm you really want to reset your policy. This will delete any custom settings you may have.")
	flags.StringVar(&options.File, "filename", "", "The policy template file containing roles and bindings.  One can be created with '"+createBootstrapPolicyCommand+"'.")
	flags.StringVar(&options.MasterConfigFile, "master-config", "openshift.local.config/master/master-config.yaml", "Location of the master configuration file to run from in order to connect to etcd and directly modify the policy.")

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

func (o OverwriteBootstrapPolicyOptions) Complete(f *clientcmd.Factory) error {
	oClient, _, err := f.Clients()
	if err != nil {
		return err
	}

	return clientcmd.Gate(oClient, "", "3.7.0")
}

func (o OverwriteBootstrapPolicyOptions) OverwriteBootstrapPolicy() error {
	masterConfig, err := configapilatest.ReadAndResolveMasterConfig(o.MasterConfigFile)
	if err != nil {
		return err
	}

	// this brings in etcd server client libraries
	optsGetter, err := originrest.StorageOptions(*masterConfig)
	if err != nil {
		return err
	}

	return OverwriteBootstrapPolicy(optsGetter, o.File, o.CreateBootstrapPolicyCommand, o.Force, o.Out)
}

type authorizationStorage struct {
	Role               role.Storage
	RoleBinding        rolebinding.Storage
	ClusterRole        clusterrole.Storage
	ClusterRoleBinding clusterrolebinding.Storage
}

func newLiveRuleResolver(policyRegistry policyregistry.Registry, policyBindingRegistry policybindingregistry.Registry, clusterPolicyRegistry clusterpolicyregistry.Registry, clusterBindingRegistry clusterpolicybindingregistry.Registry) rulevalidation.AuthorizationRuleResolver {
	return rulevalidation.NewDefaultRuleResolver(
		&policyregistry.ReadOnlyPolicyListerNamespacer{
			Registry: policyRegistry,
		},
		&policybindingregistry.ReadOnlyPolicyBindingListerNamespacer{
			Registry: policyBindingRegistry,
		},
		&clusterpolicyregistry.ReadOnlyClusterPolicyClientShim{
			ReadOnlyClusterPolicy: clusterpolicyregistry.ReadOnlyClusterPolicy{Registry: clusterPolicyRegistry},
		},
		&clusterpolicybindingregistry.ReadOnlyClusterPolicyBindingClientShim{
			ReadOnlyClusterPolicyBinding: clusterpolicybindingregistry.ReadOnlyClusterPolicyBinding{Registry: clusterBindingRegistry},
		},
	)
}

func getAuthorizationStorage(optsGetter restoptions.Getter) (*authorizationStorage, error) {
	policyStorage, err := policyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	policyBindingStorage, err := policybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	clusterPolicyStorage, err := clusterpolicyetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}
	clusterPolicyBindingStorage, err := clusterpolicybindingetcd.NewREST(optsGetter)
	if err != nil {
		return nil, err
	}

	policyRegistry := policyregistry.NewRegistry(policyStorage)
	policyBindingRegistry := policybindingregistry.NewRegistry(policyBindingStorage)
	clusterPolicyRegistry := clusterpolicyregistry.NewRegistry(clusterPolicyStorage)
	clusterPolicyBindingRegistry := clusterpolicybindingregistry.NewRegistry(clusterPolicyBindingStorage)

	liveRuleResolver := newLiveRuleResolver(policyRegistry, policyBindingRegistry, clusterPolicyRegistry, clusterPolicyBindingRegistry)

	return &authorizationStorage{
		Role:               rolestorage.NewVirtualStorage(policyRegistry, liveRuleResolver, nil),
		RoleBinding:        rolebindingstorage.NewVirtualStorage(policyBindingRegistry, liveRuleResolver, nil),
		ClusterRole:        clusterrolestorage.NewClusterRoleStorage(clusterPolicyRegistry, liveRuleResolver, nil),
		ClusterRoleBinding: clusterrolebindingstorage.NewClusterRoleBindingStorage(clusterPolicyBindingRegistry, liveRuleResolver, nil),
	}, nil
}

func OverwriteBootstrapPolicy(optsGetter restoptions.Getter, policyFile, createBootstrapPolicyCommand string, change bool, out io.Writer) error {
	if !change {
		fmt.Fprintf(out, "Performing a dry run of policy overwrite:\n\n")
	}

	mapper := kapi.Registry.RESTMapper()
	typer := kapi.Scheme
	clientMapper := resource.ClientMapperFunc(func(mapping *meta.RESTMapping) (resource.RESTClient, error) {
		return nil, nil
	})

	r := resource.NewBuilder(mapper, resource.SimpleCategoryExpander{}, typer, clientMapper, kapi.Codecs.UniversalDecoder()).
		FilenameParam(false, &resource.FilenameOptions{Recursive: false, Filenames: []string{policyFile}}).
		Flatten().
		Do()

	if r.Err() != nil {
		return r.Err()
	}

	authStorage, err := getAuthorizationStorage(optsGetter)
	if err != nil {
		return err
	}

	return r.Visit(func(info *resource.Info, err error) error {
		if err != nil {
			return err
		}
		template, ok := info.Object.(*templateapi.Template)
		if !ok {
			return errors.New("policy must be contained in a template.  One can be created with '" + createBootstrapPolicyCommand + "'.")
		}
		runtime.DecodeList(template.Objects, kapi.Codecs.UniversalDecoder())

		// For each object, we attempt the following to maximize our ability to persist the desired objects, while minimizing etcd write thrashing:
		// 1. Create the object (no-ops if the object already exists)
		// 2. If the object already exists, attempt to update the object (no-ops if an identical object is already persisted)
		// 3. If we encounter any error updating, delete and recreate
		errs := []error{}
		for _, item := range template.Objects {
			switch t := item.(type) {
			case *authorizationapi.Role:
				ctx := apirequest.WithNamespace(apirequest.NewContext(), t.Namespace)
				if change {
					// Attempt to create
					_, err := authStorage.Role.CreateRoleWithEscalation(ctx, t)
					// Unconditional replace if it already exists
					if kapierrors.IsAlreadyExists(err) {
						_, _, err = authStorage.Role.UpdateRoleWithEscalation(ctx, t)
					}
					// Delete and recreate as a last resort
					if err != nil {
						authStorage.Role.Delete(ctx, t.Name, nil)
						_, err = authStorage.Role.CreateRoleWithEscalation(ctx, t)
					}
					// Gather any error
					if err != nil {
						errs = append(errs, err)
					}
				} else {
					fmt.Fprintf(out, "Overwrite role %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRole(t); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}
			case *authorizationapi.RoleBinding:
				ctx := apirequest.WithNamespace(apirequest.NewContext(), t.Namespace)
				if change {
					// Attempt to create
					_, err := authStorage.RoleBinding.CreateRoleBindingWithEscalation(ctx, t)
					// Unconditional replace if it already exists
					if kapierrors.IsAlreadyExists(err) {
						_, _, err = authStorage.RoleBinding.UpdateRoleBindingWithEscalation(ctx, t)
					}
					// Delete and recreate as a last resort
					if err != nil {
						authStorage.RoleBinding.Delete(ctx, t.Name, nil)
						_, err = authStorage.RoleBinding.CreateRoleBindingWithEscalation(ctx, t)
					}
					// Gather any error
					if err != nil {
						errs = append(errs, err)
					}
				} else {
					fmt.Fprintf(out, "Overwrite role binding %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRoleBinding(t, nil, nil); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}

			case *authorizationapi.ClusterRole:
				ctx := apirequest.WithNamespace(apirequest.NewContext(), t.Namespace)
				if change {
					// Attempt to create
					_, err := authStorage.ClusterRole.CreateClusterRoleWithEscalation(ctx, t)
					// Unconditional replace if it already exists
					if kapierrors.IsAlreadyExists(err) {
						_, _, err = authStorage.ClusterRole.UpdateClusterRoleWithEscalation(ctx, t)
					}
					// Delete and recreate as a last resort
					if err != nil {
						authStorage.ClusterRole.Delete(ctx, t.Name, nil)
						_, err = authStorage.ClusterRole.CreateClusterRoleWithEscalation(ctx, t)
					}
					// Gather any error
					if err != nil {
						errs = append(errs, err)
					}
				} else {
					fmt.Fprintf(out, "Overwrite role %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRole(authorizationapi.ToRole(t)); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}
			case *authorizationapi.ClusterRoleBinding:
				ctx := apirequest.WithNamespace(apirequest.NewContext(), t.Namespace)
				if change {
					// Attempt to create
					_, err := authStorage.ClusterRoleBinding.CreateClusterRoleBindingWithEscalation(ctx, t)
					// Unconditional replace if it already exists
					if kapierrors.IsAlreadyExists(err) {
						_, _, err = authStorage.ClusterRoleBinding.UpdateClusterRoleBindingWithEscalation(ctx, t)
					}
					// Delete and recreate as a last resort
					if err != nil {
						authStorage.ClusterRoleBinding.Delete(ctx, t.Name, nil)
						_, err = authStorage.ClusterRoleBinding.CreateClusterRoleBindingWithEscalation(ctx, t)
					}
					// Gather any error
					if err != nil {
						errs = append(errs, err)
					}
				} else {
					fmt.Fprintf(out, "Overwrite role binding %s/%s\n", t.Namespace, t.Name)
					if s, err := describe.DescribeRoleBinding(authorizationapi.ToRoleBinding(t), nil, nil); err == nil {
						fmt.Fprintf(out, "%s\n", s)
					}
				}

			default:
				errs = append(errs, fmt.Errorf("only roles and rolebindings may be created in this mode, not: %v", reflect.TypeOf(t)))
			}
		}
		if !change {
			fmt.Fprintf(out, "To make the changes described above, pass --force\n")
		}
		return kerrors.NewAggregate(errs)
	})
}
