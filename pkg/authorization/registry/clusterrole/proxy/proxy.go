package proxy

import (
	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	"github.com/openshift/origin/pkg/authorization/registry/clusterrole"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
	rolestorage "github.com/openshift/origin/pkg/authorization/registry/role/policybased"
	"github.com/openshift/origin/pkg/authorization/rulevalidation"
)

type ClusterRoleStorage struct {
	roleStorage rolestorage.VirtualStorage
}

func NewClusterRoleStorage(clusterPolicyRegistry clusterpolicyregistry.Registry, liveRuleResolver, cachedRuleResolver rulevalidation.AuthorizationRuleResolver) clusterrole.Storage {
	return &ClusterRoleStorage{
		roleStorage: rolestorage.VirtualStorage{
			PolicyStorage: clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry),

			RuleResolver:       liveRuleResolver,
			CachedRuleResolver: cachedRuleResolver,

			CreateStrategy: roleregistry.ClusterStrategy,
			UpdateStrategy: roleregistry.ClusterStrategy,
			Resource:       authorizationapi.Resource("clusterrole"),
		},
	}
}

func (s *ClusterRoleStorage) New() runtime.Object {
	return &authorizationapi.ClusterRole{}
}
func (s *ClusterRoleStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleList{}
}

func (s *ClusterRoleStorage) List(ctx apirequest.Context, options *metainternal.ListOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.List(ctx, options)
	if ret == nil {
		return nil, err
	}
	return authorizationapi.ToClusterRoleList(ret.(*authorizationapi.RoleList)), err
}

func (s *ClusterRoleStorage) Get(ctx apirequest.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.Get(ctx, name, options)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}
func (s *ClusterRoleStorage) Delete(ctx apirequest.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	ret, immediate, err := s.roleStorage.Delete(ctx, name, options)
	if ret == nil {
		return nil, immediate, err
	}

	return ret.(*metav1.Status), false, err
}

func (s *ClusterRoleStorage) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := authorizationapi.ToRole(clusterObj)

	ret, err := s.roleStorage.Create(ctx, convertedObj, false)
	if ret == nil {
		return nil, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), err
}

type convertingObjectInfo struct {
	rest.UpdatedObjectInfo
}

func (i convertingObjectInfo) UpdatedObject(ctx apirequest.Context, old runtime.Object) (runtime.Object, error) {
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

func (s *ClusterRoleStorage) Update(ctx apirequest.Context, name string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	ret, created, err := s.roleStorage.Update(ctx, name, convertingObjectInfo{objInfo})
	if ret == nil {
		return nil, created, err
	}

	return authorizationapi.ToClusterRole(ret.(*authorizationapi.Role)), created, err
}

func (m *ClusterRoleStorage) CreateClusterRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, error) {
	in := authorizationapi.ToRole(obj)
	ret, err := m.roleStorage.CreateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), err
}

func (m *ClusterRoleStorage) UpdateClusterRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.ClusterRole) (*authorizationapi.ClusterRole, bool, error) {
	in := authorizationapi.ToRole(obj)
	ret, created, err := m.roleStorage.UpdateRoleWithEscalation(ctx, in)
	return authorizationapi.ToClusterRole(ret), created, err
}

func (m *ClusterRoleStorage) CreateRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.Role) (*authorizationapi.Role, error) {
	return m.roleStorage.CreateRoleWithEscalation(ctx, obj)
}

func (m *ClusterRoleStorage) UpdateRoleWithEscalation(ctx apirequest.Context, obj *authorizationapi.Role) (*authorizationapi.Role, bool, error) {
	return m.roleStorage.UpdateRoleWithEscalation(ctx, obj)
}
