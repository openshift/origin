package proxy

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	clusterpolicyregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicy"
	clusterpolicybindingregistry "github.com/openshift/origin/pkg/authorization/registry/clusterpolicybinding"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
	rolebindingstorage "github.com/openshift/origin/pkg/authorization/registry/rolebinding/policybased"
)

type ClusterRoleBindingStorage struct {
	roleBindingStorage rolebindingstorage.VirtualStorage
}

func NewClusterRoleBindingStorage(clusterPolicyRegistry clusterpolicyregistry.Registry, clusterBindingRegistry clusterpolicybindingregistry.Registry) *ClusterRoleBindingStorage {
	simulatedPolicyRegistry := clusterpolicyregistry.NewSimulatedRegistry(clusterPolicyRegistry)
	simulatedPolicyBindingRegistry := clusterpolicybindingregistry.NewSimulatedRegistry(clusterBindingRegistry)

	return &ClusterRoleBindingStorage{
		rolebindingstorage.VirtualStorage{
			PolicyRegistry:               simulatedPolicyRegistry,
			BindingRegistry:              simulatedPolicyBindingRegistry,
			ClusterPolicyRegistry:        clusterPolicyRegistry,
			ClusterPolicyBindingRegistry: clusterBindingRegistry,

			CreateStrategy: rolebindingregistry.ClusterStrategy,
			UpdateStrategy: rolebindingregistry.ClusterStrategy,
		},
	}
}

func (s *ClusterRoleBindingStorage) New() runtime.Object {
	return &authorizationapi.ClusterRoleBinding{}
}
func (s *ClusterRoleBindingStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleBindingList{}
}

func (s *ClusterRoleBindingStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.List(ctx, label, field)
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

	return ret.(*kapi.Status), err
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

func (s *ClusterRoleBindingStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := authorizationapi.ToRoleBinding(clusterObj)

	ret, created, err := s.roleBindingStorage.Update(ctx, convertedObj)
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
