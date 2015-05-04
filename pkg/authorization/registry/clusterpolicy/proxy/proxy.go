package proxy

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	authorizationapi "github.com/openshift/origin/pkg/authorization/api"
	policyregistry "github.com/openshift/origin/pkg/authorization/registry/policy"
)

type ClusterPolicyStorage struct {
	masterNamespace string
	typeConverter   authorizationapi.TypeConverter
	policyStorage   policyregistry.Storage
}

func NewClusterPolicyStorage(masterNamespace string, policyStorage policyregistry.Storage) *ClusterPolicyStorage {
	return &ClusterPolicyStorage{masterNamespace, authorizationapi.TypeConverter{masterNamespace}, policyStorage}
}

func (s *ClusterPolicyStorage) New() runtime.Object {
	return &authorizationapi.ClusterPolicy{}
}
func (s *ClusterPolicyStorage) NewList() runtime.Object {
	return &authorizationapi.ClusterPolicyList{}
}

func (s *ClusterPolicyStorage) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	ret, err := s.policyStorage.List(kapi.WithNamespace(ctx, s.masterNamespace), label, field)
	if ret == nil {
		return nil, err
	}
	return s.typeConverter.ToClusterPolicyList(ret.(*authorizationapi.PolicyList)), err
}

func (s *ClusterPolicyStorage) Get(ctx kapi.Context, name string) (runtime.Object, error) {
	ret, err := s.policyStorage.Get(kapi.WithNamespace(ctx, s.masterNamespace), name)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicy(ret.(*authorizationapi.Policy)), err
}
func (s *ClusterPolicyStorage) Delete(ctx kapi.Context, name string, options *kapi.DeleteOptions) (runtime.Object, error) {
	ret, err := s.policyStorage.Delete(kapi.WithNamespace(ctx, s.masterNamespace), name, options)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicy(ret.(*authorizationapi.Policy)), err
}

func (s *ClusterPolicyStorage) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	clusterObj := obj.(*authorizationapi.ClusterPolicy)
	convertedObj := s.typeConverter.ToPolicy(clusterObj)

	ret, err := s.policyStorage.Create(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, err
	}

	return s.typeConverter.ToClusterPolicy(ret.(*authorizationapi.Policy)), err
}

func (s *ClusterPolicyStorage) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	clusterObj := obj.(*authorizationapi.ClusterPolicy)
	convertedObj := s.typeConverter.ToPolicy(clusterObj)

	ret, created, err := s.policyStorage.Update(kapi.WithNamespace(ctx, s.masterNamespace), convertedObj)
	if ret == nil {
		return nil, created, err
	}

	return s.typeConverter.ToClusterPolicy(ret.(*authorizationapi.Policy)), created, err
}
