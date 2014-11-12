package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeBuildStore struct {
	Build *buildapi.Build
}

func NewFakeBuildStore(build *buildapi.Build) FakeBuildStore {
	return FakeBuildStore{build}
}

func (s FakeBuildStore) Add(id string, obj interface{}) {
}

func (s FakeBuildStore) Update(id string, obj interface{}) {
}

func (s FakeBuildStore) Delete(id string) {
}

func (s FakeBuildStore) List() []interface{} {
	return []interface{}{s.Build}
}

func (s FakeBuildStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}

func (s FakeBuildStore) Get(id string) (item interface{}, exists bool) {
	if s.Build == nil {
		return nil, false
	}

	return s.Build, true
}

func (s FakeBuildStore) Replace(idToObj map[string]interface{}) {}
