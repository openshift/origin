package rolebinding

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	restclient "k8s.io/client-go/rest"
	rbacinternalversion "k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/typed/rbac/internalversion"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	"github.com/openshift/origin/pkg/authorization/registry/util"
	authclient "github.com/openshift/origin/pkg/client/impersonatingclient"
	utilregistry "github.com/openshift/origin/pkg/util/registry"
)

type REST struct {
	privilegedClient restclient.Interface
}

var _ rest.Lister = &REST{}
var _ rest.Getter = &REST{}
var _ rest.CreaterUpdater = &REST{}
var _ rest.GracefulDeleter = &REST{}

func NewREST(client restclient.Interface) utilregistry.NoWatchStorage {
	return utilregistry.WrapNoWatchStorageError(&REST{privilegedClient: client})
}

func (s *REST) New() runtime.Object {
	return &authorizationapi.RoleBinding{}
}
func (s *REST) NewList() runtime.Object {
	return &authorizationapi.RoleBindingList{}
}

func (s *REST) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	client, err := s.getImpersonatingClient(ctx)
	if err != nil {
		return nil, err
	}

	optv1 := metav1.ListOptions{}
	if err := metainternal.Convert_internalversion_ListOptions_To_v1_ListOptions(options, &optv1, nil); err != nil {
		return nil, err
	}

	bindings, err := client.List(optv1)
	if err != nil {
		return nil, err
	}

	ret := &authorizationapi.RoleBindingList{}
	for _, curr := range bindings.Items {
		role, err := util.RoleBindingFromRBAC(&curr)
		if err != nil {
			return nil, err
		}
		ret.Items = append(ret.Items, *role)
	}
	ret.ListMeta.ResourceVersion = bindings.ResourceVersion
	return ret, nil
}

func (s *REST) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	client, err := s.getImpersonatingClient(ctx)
	if err != nil {
		return nil, err
	}

	ret, err := client.Get(name, *options)
	if err != nil {
		return nil, err
	}

	binding, err := util.RoleBindingFromRBAC(ret)
	if err != nil {
		return nil, err
	}
	return binding, nil
}

func (s *REST) Delete(ctx apirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	client, err := s.getImpersonatingClient(ctx)
	if err != nil {
		return nil, false, err
	}

	if err := client.Delete(name, options); err != nil {
		return nil, false, err
	}

	return &metav1.Status{Status: metav1.StatusSuccess}, true, nil
}

func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ rest.ValidateObjectFunc, _ bool) (runtime.Object, error) {
	client, err := s.getImpersonatingClient(ctx)
	if err != nil {
		return nil, err
	}

	rb := obj.(*authorizationapi.RoleBinding)

	// Default the namespace if it is not specified so conversion does not error
	// Normally this is done during the REST strategy but we avoid those here to keep the proxies simple
	if ns, ok := apirequest.NamespaceFrom(ctx); ok && len(ns) > 0 && len(rb.Namespace) == 0 && len(rb.RoleRef.Namespace) > 0 {
		deepcopiedObj := rb.DeepCopy()
		deepcopiedObj.Namespace = ns
		rb = deepcopiedObj
	}

	convertedObj, err := util.RoleBindingToRBAC(rb)
	if err != nil {
		return nil, err
	}

	ret, err := client.Create(convertedObj)
	if err != nil {
		return nil, err
	}

	binding, err := util.RoleBindingFromRBAC(ret)
	if err != nil {
		return nil, err
	}
	return binding, nil
}

func (s *REST) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo, _ rest.ValidateObjectFunc, _ rest.ValidateObjectUpdateFunc) (runtime.Object, bool, error) {
	client, err := s.getImpersonatingClient(ctx)
	if err != nil {
		return nil, false, err
	}

	old, err := client.Get(name, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	oldRoleBinding, err := util.RoleBindingFromRBAC(old)
	if err != nil {
		return nil, false, err
	}

	obj, err := objInfo.UpdatedObject(ctx, oldRoleBinding)
	if err != nil {
		return nil, false, err
	}

	updatedRoleBinding, err := util.RoleBindingToRBAC(obj.(*authorizationapi.RoleBinding))
	if err != nil {
		return nil, false, err
	}

	ret, err := client.Update(updatedRoleBinding)
	if err != nil {
		return nil, false, err
	}

	role, err := util.RoleBindingFromRBAC(ret)
	if err != nil {
		return nil, false, err
	}
	return role, false, err
}

func (s *REST) getImpersonatingClient(ctx apirequest.Context) (rbacinternalversion.RoleBindingInterface, error) {
	namespace, ok := apirequest.NamespaceFrom(ctx)
	if !ok {
		return nil, apierrors.NewBadRequest("namespace parameter required")
	}
	rbacClient, err := authclient.NewImpersonatingRBACFromContext(ctx, s.privilegedClient)
	if err != nil {
		return nil, err
	}
	return rbacClient.RoleBindings(namespace), nil
}
