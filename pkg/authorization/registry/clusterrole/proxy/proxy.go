package proxy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type ClusterRoleStorage struct {
	roleStorage rolestorage.VirtualStorage
}

func NewClusterRoleStorage(clusterPolicyRegistry clusterpolicyregistry.Registry, clusterBindingRegistry clusterpolicybindingregistry.Registry, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) *ClusterRoleStorage {
	simulatedPolicyRegistry := clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry)

	ruleResolver := rulevalidation.NewDefaultRuleResolver(
		nil,
		nil,
		clusterpolicyregistry.ReadOnlyClusterPolicy{Registry: clusterPolicyRegistry},
		clusterpolicybindingregistry.ReadOnlyClusterPolicyBinding{Registry: clusterBindingRegistry},
	)

	return &ClusterRoleStorage{
		roleStorage: rolestorage.VirtualStorage{
			PolicyStorage: simulatedPolicyRegistry,

			RuleResolver:       ruleResolver,
			CachedRuleResolver: cachedRuleResolver,

			CreateStrategy: roleregistry.ClusterStrategy,
			UpdateStrategy: roleregistry.ClusterStrategy,
			Resource:       authorizationapi.Resource("clusterrole")},
	}
}

func (s *ClusterRoleStorage) New() runtime.Object {
	return &authorizationapi.ClusterRole{}
}
func (s *ClusterRoleStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleList{}
}

func (s *ClusterRoleStorage) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.List(ctx, options)
	if ret == nil {
		return nil, err
	}
	return authorizationapi.ToClusterRoleList(ret.(*authorizationapi.RoleList)), err
}

func (s *ClusterRoleStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.roleStorage.Get(ctx, name)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}
func (s *ClusterRoleStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.Delete(ctx, name, options)
	if ret == nil {
		return nil, err
	}

	return ret.(*unversioned.Status), err
}

func (s *ClusterRoleStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := authorizationapi.ToRole(clusterObj)

	ret, err := s.roleStorage.Create(ctx, convertedObj)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}

type convertingObjectInfo struct {
	rest.UpdatedObjectInfo
}

func (i convertingObjectInfo) UpdatedObject(ctx kapi.Context, old runtime.Object) (runtime.Object, error) {
	oldObj := old.(*authorizationapi.Role)
	convertedOldObj := authorizationapi.ToClusterRole(oldObj)
	obj, err := i.UpdatedObjectInfo.UpdatedObject(ctx, convertedOldObj)
	if err != nil {
		return nil, err
	}
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := authorizationapi.ToRole(clusterObj)
	return convertedObj, nil
}

func (s *ClusterRoleStorage) Update(ctx kapi.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	ret, created, err := s.roleStorage.Update(ctx, name, convertingObjectInfo{objInfo})
	if ret == nil {
		return nil, created, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), created, err
}

func (m *ClusterRoleStorage) CreateClusterRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error) {
	in := authorizationapi.ToRole(obj)
	ret, err := m.roleStorage.CreateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), err
}

func (m *ClusterRoleStorage) UpdateClusterRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error) {
	in := authorizationapi.ToRole(obj)
	ret, created, err := m.roleStorage.UpdateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), created, err
}

func (m *ClusterRoleStorage) CreateRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.Role) (*authorizationapi.Role, error) {
	return m.roleStorage.CreateRoleWithEscalation(ctx, obj)
}

func (m *ClusterRoleStorage) UpdateRoleWithEscalation(ctx kapi.Context, obj *authorizationapi.Role) (*authorizationapi.Role, bool, error) {
	return m.roleStorage.UpdateRoleWithEscalation(ctx, obj)
}
