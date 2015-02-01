package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	buildapi "github.com/openshift/origin/pkg/build/api"
)

type FakeBuildStore struct {
	Build *buildapi.Build
	Err   error
}

func NewFakeBuildStore(build *buildapi.Build) FakeBuildStore {
	return FakeBuildStore{Build: build}
}

func (s FakeBuildStore) Add(obj interface{}) error {
	return s.Err
}

func (s FakeBuildStore) Update(obj interface{}) error {
	return s.Err
}

func (s FakeBuildStore) Delete(obj interface{}) error {
	return s.Err
}

func (s FakeBuildStore) List() []interface{} {
	return []interface{}{s.Build}
}

func (s FakeBuildStore) ContainedIDs() util.StringSet {
	return util.NewStringSet()
}

func (s FakeBuildStore) Get(obj interface{}) (item interface{}, exists bool, err error) {
	return s.GetByKey("")
}

func (s FakeBuildStore) GetByKey(id string) (item interface{}, exists bool, err error) {
	if s.Err != nil {
		return nil, false, err
	}
	if s.Build == nil {
		return nil, false, nil
	}

	return s.Build, true, nil
}

func (s FakeBuildStore) Replace(list []interface{}) error {
	return nil
}
