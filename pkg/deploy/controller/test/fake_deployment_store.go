package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
)

type FakeDeploymentStore struct {
	Deployment *kapi.ReplicationController
}

func NewFakeDeploymentStore(deployment *kapi.ReplicationController) FakeDeploymentStore {
	return FakeDeploymentStore{deployment}
}

func (s FakeDeploymentStore) Add(id string, obj interface{})    {}
func (s FakeDeploymentStore) Update(id string, obj interface{}) {}
func (s FakeDeploymentStore) Delete(id string)                  {}
func (s FakeDeploymentStore) List() []interface{} {
	return []interface{}{s.Deployment}
}
func (s FakeDeploymentStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}
func (s FakeDeploymentStore) Get(id string) (item interface{}, exists bool) {
	if s.Deployment == nil {
		return nil, false
	}

	return s.Deployment, true
}
func (s FakeDeploymentStore) Replace(idToObj map[string]interface{}) {}
