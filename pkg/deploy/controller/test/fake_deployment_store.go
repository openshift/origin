package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type FakeDeploymentStore struct {
	Deployment *deployapi.Deployment
}

func NewFakeDeploymentStore(deployment *deployapi.Deployment) FakeDeploymentStore {
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
