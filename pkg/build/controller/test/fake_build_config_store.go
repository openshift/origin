package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeBuildConfigStore struct {
	BuildConfig *buildapi.BuildConfig
}

func NewFakeBuildConfigStore(buildcfg *buildapi.BuildConfig) FakeBuildConfigStore {
	return FakeBuildConfigStore{buildcfg}
}

func (s FakeBuildConfigStore) Add(id string, obj interface{}) {
}

func (s FakeBuildConfigStore) Update(id string, obj interface{}) {
}

func (s FakeBuildConfigStore) Delete(id string) {
}

func (s FakeBuildConfigStore) List() []interface{} {
	return []interface{}{s.BuildConfig}
}

func (s FakeBuildConfigStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}

func (s FakeBuildConfigStore) Get(id string) (item interface{}, exists bool) {
	if s.BuildConfig == nil {
		return nil, false
	}

	return s.BuildConfig, true
}

func (s FakeBuildConfigStore) Replace(idToObj map[string]interface{}) {}
