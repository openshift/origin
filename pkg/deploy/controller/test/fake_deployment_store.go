package test

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/util"
)

type FakeDeploymentStore struct {
	Deployment *kapi.ReplicationController
	Err        error
}

func NewFakeDeploymentStore(deployment *kapi.ReplicationController) FakeDeploymentStore {
	return FakeDeploymentStore{Deployment: deployment}
}

func (s FakeDeploymentStore) Add(obj interface{}) error {
	return s.Err
}

func (s FakeDeploymentStore) Update(obj interface{}) error {
	return s.Err
}

func (s FakeDeploymentStore) Delete(obj interface{}) error {
	return s.Err
}

func (s FakeDeploymentStore) List() []interface{} {
	return []interface{}{s.Deployment}
}

func (s FakeDeploymentStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}

func (s FakeDeploymentStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return s.GetByKey("")
}

func (s FakeDeploymentStore) GetByKey(id string) (item interface{}, exists bool, err error) {
	if s.Err != nil {
		return nil, false, err
	}
	if s.Deployment == nil {
		return nil, false, nil
	}

	return s.Deployment, true, nil
}

func (s FakeDeploymentStore) Replace(list []interface{}) error {
	return nil
}
