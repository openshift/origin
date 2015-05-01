package proxy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	rolebindingregistry "github.com/openshift/origin/pkg/authorization/registry/rolebinding"
)

type ClusterRoleBindingStorage struct {
	masterNamespace    string
	typeConverter      authorizationapi.TypeConverter
	roleBindingStorage rolebindingregistry.Storage
}

func NewClusterRoleBindingStorage(masterNamespace string, roleBindingStorage rolebindingregistry.Storage) *ClusterRoleBindingStorage {
	return &ClusterRoleBindingStorage{masterNamespace, authorizationapi.TypeConverter{masterNamespace}, roleBindingStorage}
}

func (s *ClusterRoleBindingStorage) New() runtime.Object {
	return &authorizationapi.ClusterRoleBinding{}
}
func (s *ClusterRoleBindingStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleBindingList{}
}

func (s *ClusterRoleBindingStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.List(kapi.WithNamespace(ctx, s.masterNamespace), label, field)
	if ret == nil {
		return nil, err
	}
	return s.typeConverter.ToClusterRoleBindingList(ret.(*authorizationapi.RoleBindingList)), err
}

func (s *ClusterRoleBindingStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.Get(kapi.WithNamespace(ctx, s.masterNamespace), name)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}
func (s *ClusterRoleBindingStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.roleBindingStorage.Delete(kapi.WithNamespace(ctx, s.masterNamespace), name, options)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}

func (s *ClusterRoleBindingStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := s.typeConverter.ToRoleBinding(clusterObj)

	ret, err := s.roleBindingStorage.Create(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), err
}

func (s *ClusterRoleBindingStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterRoleBinding)
	convertedObj := s.typeConverter.ToRoleBinding(clusterObj)

	ret, created, err := s.roleBindingStorage.Update(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, created, err
	}

	return s.typeConverter.ToClusterRoleBinding(ret.(*authorizationapi.RoleBinding)), created, err
}
