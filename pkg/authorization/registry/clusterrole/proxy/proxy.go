package proxy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	roleregistry "github.com/openshift/origin/pkg/authorization/registry/role"
)

type ClusterRoleStorage struct {
	masterNamespace string
	typeConverter   authorizationapi.TypeConverter
	roleStorage     roleregistry.Storage
}

func NewClusterRoleStorage(masterNamespace string, roleStorage roleregistry.Storage) *ClusterRoleStorage {
	return &ClusterRoleStorage{masterNamespace, authorizationapi.TypeConverter{masterNamespace}, roleStorage}
}

func (s *ClusterRoleStorage) New() runtime.Object {
	return &authorizationapi.ClusterRole{}
}
func (s *ClusterRoleStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterRoleList{}
}

func (s *ClusterRoleStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	ret, err := s.roleStorage.List(kapi.WithNamespace(ctx, s.masterNamespace), label, field)
	if ret == nil {
		return nil, err
	}
	return s.typeConverter.ToClusterRoleList(ret.(*authorizationapi.RoleList)), err
}

func (s *ClusterRoleStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.roleStorage.Get(kapi.WithNamespace(ctx, s.masterNamespace), name)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRole(ret.(*authorizationapi.Role)), err
}
func (s *ClusterRoleStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.roleStorage.Delete(kapi.WithNamespace(ctx, s.masterNamespace), name, options)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRole(ret.(*authorizationapi.Role)), err
}

func (s *ClusterRoleStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := s.typeConverter.ToRole(clusterObj)

	ret, err := s.roleStorage.Create(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterRole(ret.(*authorizationapi.Role)), err
}

func (s *ClusterRoleStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterRole)
	convertedObj := s.typeConverter.ToRole(clusterObj)

	ret, created, err := s.roleStorage.Update(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, created, err
	}

	return s.typeConverter.ToClusterRole(ret.(*authorizationapi.Role)), created, err
}
