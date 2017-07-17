package proxy

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrolebinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type ClusterRoleBindingStorage struct {
	roleBindingStorage rolebindingstorage.VirtualStorage
}

func NewClusterRoleBindingStorage(clusterBindingRegistry clusterpolicybindingregistry.Registry, liveRuleResolver, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) clusterrolebinding.Storage {
	return &ClusterRoleBindingStorage{
		roleBindingStorage: rolebindingstorage.VirtualStorage{
			BindingRegistry: clusterpolicybindingregistry.NewSimulatedRegistry(clusterBindingRegistry),

			RuleResolver:       liveRuleResolver,
			CachedRuleResolver: cachedRuleResolver,

			CreateStrategy: rolebindingregistry.ClusterStrategy,
			UpdateStrategy: rolebindingregistry.ClusterStrategy,
			Resource:       authorizationapi.Resource("clusterrolebinding"),
		},
	}
}

func (s *ClusterRoleBindingStorage) New() runtime.Object {
	return &authorizationapi.ClusterRoleBinding{}
}
func (s *ClusterRoleBindingStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleBindingList{}
}

func (s *ClusterRoleBindingStorage) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.List(ctx, options)
	if ret == nil {
		return nil, err
	}
	return authorizationapi.ToClusterRoleBindingList(ret.(*authorizationapi.RoleBindingList)), err
}

func (s *ClusterRoleBindingStorage) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.Get(ctx, name, options)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}
func (s *ClusterRoleBindingStorage) Delete(ctx apirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	ret, immediate, err := s.roleBindingStorage.Delete(ctx, name, options)
	if ret == nil {
		return nil, immediate, err
	}

	return ret.(*metav1.Status), false, err
}

func (s *ClusterRoleBindingStorage) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := authorizationapi.ToRoleBinding(clusterObj)

	ret, err := s.roleBindingStorage.Create(ctx, convertedObj, false)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}

type convertingObjectInfo struct {
	rest.UpdatedObjectInfo
}

func (i convertingObjectInfo) UpdatedObject(ctx apirequest.Context, old runtime.Object) (runtime.Object, error) {
	oldObj := old.(*authorizationapi.RoleBinding)
	convertedOldObj := authorizationapi.ToClusterRoleBinding(oldObj)
	obj, err := i.UpdatedObjectInfo.UpdatedObject(ctx, convertedOldObj)
	if err != nil {
		return nil, err
	}
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := authorizationapi.ToRoleBinding(clusterObj)
	return convertedObj, nil
}

func (s *ClusterRoleBindingStorage) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	ret, created, err := s.roleBindingStorage.Update(ctx, name, convertingObjectInfo{objInfo})
	if ret == nil {
		return nil, created, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), created, err
}

func (m *ClusterRoleBindingStorage) CreateClusterRoleBindingWithEscalation(ctx apirequest.Context, obj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	in := authorizationapi.ToRoleBinding(obj)
	ret, err := m.roleBindingStorage.CreateRoleBindingWithEscalation(ctx, in)
	return authorizationapi.ToClusterRoleBinding(ret), err
}

func (m *ClusterRoleBindingStorage) UpdateClusterRoleBindingWithEscalation(ctx apirequest.Context, obj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error) {
	in := authorizationapi.ToRoleBinding(obj)
	ret, created, err := m.roleBindingStorage.UpdateRoleBindingWithEscalation(ctx, in)
	return authorizationapi.ToClusterRoleBinding(ret), created, err
}
