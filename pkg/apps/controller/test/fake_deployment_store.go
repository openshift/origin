package test

import (
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/sets"
)

type FakeDeploymentStore struct {
	Deployment *v1.ReplicationController
	Err        error
}

func NewFakeDeploymentStore(deployment *v1.ReplicationController) FakeDeploymentStore {
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

func (s FakeDeploymentStore) ContainedIDs() sets.String {
	return sets.NewString()
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
