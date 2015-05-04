package proxy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policybindingregistry "github.com/openshift/origin/pkg/authorization/registry/policybinding"
)

type ClusterPolicyBindingStorage struct {
	masterNamespace      string
	typeConverter        authorizationapi.TypeConverter
	policyBindingStorage policybindingregistry.Storage
}

func NewClusterPolicyBindingStorage(masterNamespace string, policyBindingStorage policybindingregistry.Storage) *ClusterPolicyBindingStorage {
	return &ClusterPolicyBindingStorage{masterNamespace, authorizationapi.TypeConverter{masterNamespace}, policyBindingStorage}
}

func (s *ClusterPolicyBindingStorage) New() runtime.Object {
	return &authorizationapi.ClusterPolicyBinding{}
}
func (s *ClusterPolicyBindingStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterPolicyBindingList{}
}

func (s *ClusterPolicyBindingStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	ret, err := s.policyBindingStorage.List(kapi.WithNamespace(ctx, s.masterNamespace), label, field)
	if ret == nil {
		return nil, err
	}
	return s.typeConverter.ToClusterPolicyBindingList(ret.(*authorizationapi.PolicyBindingList)), err
}

func (s *ClusterPolicyBindingStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.policyBindingStorage.Get(kapi.WithNamespace(ctx, s.masterNamespace), name)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicyBinding(ret.(*authorizationapi.PolicyBinding)), err
}
func (s *ClusterPolicyBindingStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.policyBindingStorage.Delete(kapi.WithNamespace(ctx, s.masterNamespace), name, options)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicyBinding(ret.(*authorizationapi.PolicyBinding)), err
}

func (s *ClusterPolicyBindingStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterPolicyBinding)
	convertedObj := s.typeConverter.ToPolicyBinding(clusterObj)

	ret, err := s.policyBindingStorage.Create(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicyBinding(ret.(*authorizationapi.PolicyBinding)), err
}

func (s *ClusterPolicyBindingStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterPolicyBinding)
	convertedObj := s.typeConverter.ToPolicyBinding(clusterObj)

	ret, created, err := s.policyBindingStorage.Update(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, created, err
	}

	return s.typeConverter.ToClusterPolicyBinding(ret.(*authorizationapi.PolicyBinding)), created, err
}
