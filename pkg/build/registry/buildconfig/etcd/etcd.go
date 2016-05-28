package etcd

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/auth/user"
	"k8s.io/kubernetes/pkg/client/restclient"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
	etcdgeneric "k8s.io/kubernetes/pkg/registry/generic/etcd"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/storage"
	"k8s.io/kubernetes/pkg/util/sets"

	authenticationapi "github.com/openshift/origin/pkg/auth/api"
	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/registry/buildconfig"
	oclient "github.com/openshift/origin/pkg/client"

	// refactor to avoid wacky dependency
	"github.com/openshift/origin/pkg/cmd/admin/policy"
)

type REST struct {
	*etcdgeneric.Etcd

	// TODO make this an index from namespace/username to roles
	roleBindingClient oclient.RoleBindingsNamespacer

	restClient *restclient.RESTClient
}

// NewStorage returns a RESTStorage object that will work against nodes.
func NewREST(s storage.Interface, roleBindingClient oclient.RoleBindingsNamespacer, restClient *restclient.RESTClient) *REST {
	prefix := "/buildconfigs"

	store := &etcdgeneric.Etcd{
		NewFunc:           func() runtime.Object { return &api.BuildConfig{} },
		NewListFunc:       func() runtime.Object { return &api.BuildConfigList{} },
		QualifiedResource: api.Resource("buildconfigs"),
		KeyRootFunc: func(ctx kapi.Context) string {
			return etcdgeneric.NamespaceKeyRootFunc(ctx, prefix)
		},
		KeyFunc: func(ctx kapi.Context, id string) (string, error) {
			return etcdgeneric.NamespaceKeyFunc(ctx, prefix, id)
		},
		ObjectNameFunc: func(obj runtime.Object) (string, error) {
			return obj.(*api.BuildConfig).Name, nil
		},
		PredicateFunc: func(label labels.Selector, field fields.Selector) generic.Matcher {
			return buildconfig.Matcher(label, field)
		},

		CreateStrategy:      buildconfig.Strategy,
		UpdateStrategy:      buildconfig.Strategy,
		DeleteStrategy:      buildconfig.Strategy,
		ReturnDeletedObject: false,
		Storage:             s,
	}

	return &REST{
		Etcd:              store,
		roleBindingClient: roleBindingClient,
		restClient:        restClient,
	}
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	bc, ok := obj.(*api.BuildConfig)
	if !ok {
		return r.Etcd.Create(ctx, obj)
	}
	if bc.Spec.Strategy.JenkinsPipelineStrategy == nil {
		return r.Etcd.Create(ctx, obj)
	}
	user, ok := kapi.UserFrom(ctx)
	if !ok {
		return r.Etcd.Create(ctx, obj)
	}

	// otherwise, find the jenkins SA specified (is that missing) and get roles it has in this namespace
	roleBindings, err := r.roleBindingClient.RoleBindings(bc.Namespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, kapierrors.NewInternalError(err)
	}
	foundEdit := false
	for _, roleBinding := range roleBindings.Items {
		if len(roleBinding.RoleRef.Namespace) != 0 || roleBinding.RoleRef.Name != "edit" {
			continue
		}
		for _, subject := range roleBinding.Subjects {
			if subject.Kind == "ServiceAccount" && subject.Name == "jenkins" && subject.Namespace == bc.Namespace {
				foundEdit = true
			}
		}
	}

	// we didn't find the role we wanted, try to add that role
	if !foundEdit {
		addRole := &policy.RoleModificationOptions{
			RoleName: "edit",
			RoleBindingAccessor: &impersonatingRoleBindingAccessor{
				user:              user,
				bindingNamespace:  bc.Namespace,
				roleBindingClient: r.roleBindingClient,
				restClient:        r.restClient,
			},
			Subjects: []kapi.ObjectReference{{Kind: "ServiceAccount", Namespace: bc.Namespace, Name: "jenkins"}},
		}

		if err := addRole.AddRole(); err != nil {
			if kapierrors.IsForbidden(err) {
				return nil, kapierrors.NewInternalError(fmt.Errorf("need to bind jenkins to edit: `oc policy add-role-to-user edit -z jenkins`"))
			}
			return nil, kapierrors.NewInternalError(err)
		}
	}

	return r.Etcd.Create(ctx, obj)
}

type impersonatingRoleBindingAccessor struct {
	user             user.Info
	bindingNamespace string

	roleBindingClient oclient.RoleBindingsNamespacer

	// TODO pretty this up
	restClient *restclient.RESTClient
}

func (a impersonatingRoleBindingAccessor) GetExistingRoleBindingsForRole(roleNamespace, role string) ([]*authorizationapi.RoleBinding, error) {
	existingBindings, err := a.roleBindingClient.RoleBindings(a.bindingNamespace).List(kapi.ListOptions{})
	if err != nil && !kapierrors.IsNotFound(err) {
		return nil, err
	}

	ret := make([]*authorizationapi.RoleBinding, 0)
	// see if we can find an existing binding that points to the role in question.
	for i := range existingBindings.Items {
		currBinding := &existingBindings.Items[i]
		if currBinding.RoleRef.Namespace == roleNamespace && currBinding.RoleRef.Name == role {
			t := currBinding
			ret = append(ret, t)
		}
	}

	return ret, nil
}

func (a impersonatingRoleBindingAccessor) GetExistingRoleBindingNames() (*sets.String, error) {
	roleBindings, err := a.roleBindingClient.RoleBindings(a.bindingNamespace).List(kapi.ListOptions{})
	if err != nil {
		return nil, err
	}

	ret := &sets.String{}
	for _, currBinding := range roleBindings.Items {
		ret.Insert(currBinding.Name)
	}

	return ret, nil
}

func (a impersonatingRoleBindingAccessor) UpdateRoleBinding(binding *authorizationapi.RoleBinding) error {
	request := a.restClient.Put()
	request = request.SetHeader(authenticationapi.ImpersonateUserHeader, a.user.GetName())
	// TODO need Impersonate-User-Groups
	for _, scope := range a.user.GetExtra()[authorizationapi.ScopesKey] {
		request = request.SetHeader(authenticationapi.ImpersonateUserScopeHeader, scope)
	}

	_, err := request.Namespace(a.bindingNamespace).Resource("roleBindings").Name(binding.Name).Body(binding).Do().Get()
	return err
}

func (a impersonatingRoleBindingAccessor) CreateRoleBinding(binding *authorizationapi.RoleBinding) error {
	binding.Namespace = a.bindingNamespace

	request := a.restClient.Post()
	request = request.SetHeader(authenticationapi.ImpersonateUserHeader, a.user.GetName())
	// TODO need Impersonate-User-Groups
	for _, scope := range a.user.GetExtra()[authorizationapi.ScopesKey] {
		request = request.SetHeader(authenticationapi.ImpersonateUserScopeHeader, scope)
	}

	_, err := request.Namespace(a.bindingNamespace).Resource("roleBindings").Body(binding).Do().Get()
	return err
}
