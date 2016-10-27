package proxy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type ClusterRoleBindingStorage struct {
	roleBindingStorage rolebindingstorage.VirtualStorage
}

func NewClusterRoleBindingStorage(clusterPolicyRegistry clusterpolicyregistry.Registry, clusterPolicyBindingRegistry clusterpolicybindingregistry.Registry, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) *ClusterRoleBindingStorage {
	simulatedPolicyBindingRegistry := clusterpolicybindingregistry.NewSimulatedRegistry(clusterPolicyBindingRegistry)

	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		nil,
		nil,
		clusterpolicyregistry.ReadOnlyClusterPolicy{Registry: clusterPolicyRegistry},
		clusterpolicybindingregistry.ReadOnlyClusterPolicyBinding{Registry: clusterPolicyBindingRegistry},
	)

	return &ClusterRoleBindingStorage{
		rolebindingstorage.VirtualStorage{
			BindingRegistry: simulatedPolicyBindingRegistry,

			RuleResolver:       ruleResolver,
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

func (s *ClusterRoleBindingStorage) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.List(ctx, options)
	if ret == nil {
		return nil, err
	}
	return authorizationapi.ToClusterRoleBindingList(ret.(*authorizationapi.RoleBindingList)), err
}

func (s *ClusterRoleBindingStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.Get(ctx, name)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}
func (s *ClusterRoleBindingStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.Delete(ctx, name, options)
	if ret == nil {
		return nil, err
	}

	return ret.(*unversioned.Status), err
}

func (s *ClusterRoleBindingStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := authorizationapi.ToRoleBinding(clusterObj)

	ret, err := s.roleBindingStorage.Create(ctx, convertedObj)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}

type convertingObjectInfo struct {
	rest.UpdatedObjectInfo
}

func (i convertingObjectInfo) UpdatedObject(ctx kapi.Context, old runtime.Object) (runtime.Object, error) {
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

func (s *ClusterRoleBindingStorage) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	ret, created, err := s.roleBindingStorage.Update(ctx, name, convertingObjectInfo{objInfo})
	if ret == nil {
		return nil, created, err
	}

	return authorizationapi.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), created, err
}

func (m *ClusterRoleBindingStorage) CreateClusterRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, error) {
	in := authorizationapi.ToRoleBinding(obj)
	ret, err := m.roleBindingStorage.CreateRoleBindingWithEscalation(ctx, in)
	return authorizationapi.ToClusterRoleBinding(ret), err
}

func (m *ClusterRoleBindingStorage) UpdateClusterRoleBindingWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRoleBinding) (*authorizationapi.ClusterRoleBinding, bool, error) {
	in := authorizationapi.ToRoleBinding(obj)
	ret, created, err := m.roleBindingStorage.UpdateRoleBindingWithEscalation(ctx, in)
	return authorizationapi.ToClusterRoleBinding(ret), created, err
}
