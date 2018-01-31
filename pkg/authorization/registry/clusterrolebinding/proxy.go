package clusterrolebinding

import (
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
	return &authorizationapi.ClusterRoleBinding{}
}
func (s *REST) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleBindingList{}
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

	ret := &authorizationapi.ClusterRoleBindingList{}
	for _, curr := range bindings.Items {
		role, err := util.ClusterRoleBindingFromRBAC(&curr)
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

	binding, err := util.ClusterRoleBindingFromRBAC(ret)
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

	convertedObj, err := util.ClusterRoleBindingToRBAC(obj.(*authorizationapi.ClusterRoleBinding))
	if err != nil {
		return nil, err
	}

	ret, err := client.Create(convertedObj)
	if err != nil {
		return nil, err
	}

	binding, err := util.ClusterRoleBindingFromRBAC(ret)
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

	oldRoleBinding, err := util.ClusterRoleBindingFromRBAC(old)
	if err != nil {
		return nil, false, err
	}

	obj, err := objInfo.UpdatedObject(ctx, oldRoleBinding)
	if err != nil {
		return nil, false, err
	}

	updatedRoleBinding, err := util.ClusterRoleBindingToRBAC(obj.(*authorizationapi.ClusterRoleBinding))
	if err != nil {
		return nil, false, err
	}

	ret, err := client.Update(updatedRoleBinding)
	if err != nil {
		return nil, false, err
	}

	role, err := util.ClusterRoleBindingFromRBAC(ret)
	if err != nil {
		return nil, false, err
	}
	return role, false, err
}

func (s *REST) getImpersonatingClient(ctx apirequest.Context) (rbacinternalversion.ClusterRoleBindingInterface, error) {
	rbacClient, err := authclient.NewImpersonatingRBACFromContext(ctx, s.privilegedClient)
	if err != nil {
		return nil, err
	}
	return rbacClient.ClusterRoleBindings(), nil
}
