package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type FakeDeploymentConfigStore struct {
	DeploymentConfig *deployapi.DeploymentConfig
}

func NewFakeDeploymentConfigStore(config *deployapi.DeploymentConfig) FakeDeploymentConfigStore {
	return FakeDeploymentConfigStore{config}
}

func (s FakeDeploymentConfigStore) Add(id string, obj interface{})    {}
func (s FakeDeploymentConfigStore) Update(id string, obj interface{}) {}
func (s FakeDeploymentConfigStore) Delete(id string)                  {}
func (s FakeDeploymentConfigStore) List() []interface{} {
	return []interface{}{s.DeploymentConfig}
}
func (s FakeDeploymentConfigStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}
func (s FakeDeploymentConfigStore) Get(id string) (item interface{}, exists bool) {
	return nil, false
}
func (s FakeDeploymentConfigStore) Replace(idToObj map[string]interface{}) {}
