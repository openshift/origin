package test

import (
	buildapi "github.com/openshift/origin/pkg/build/api"
	"k8s.io/kubernetes/pkg/util/sets"
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

func (s FakeBuildStore) Resync() error {
	return s.Err
}

func (s FakeBuildStore) List() []interface{} {
	return []interface{}{s.Build}
}

func (s FakeBuildStore) ListKeys() []string {
	return []string{"build"}
}

func (s FakeBuildStore) ContainedIDs() sets.String {
	return sets.NewString()
}

func (s FakeBuildStore) Get(obj interface{}) (interface{}, bool, error) {
	return s.GetByKey("")
}

func (s FakeBuildStore) GetByKey(id string) (interface{}, bool, error) {
	if s.Err != nil {
		return nil, false, s.Err
	}
	if s.Build == nil {
		return nil, false, nil
	}

	return s.Build, true, nil
}

func (s FakeBuildStore) Replace(list []interface{}, resourceVersion string) error {
	return nil
}
