package test

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/openshift/origin/pkg/build/api"
)

type BuildRegistry struct {
	Err            error
	Builds         *api.BuildList
	Build          *api.Build
	DeletedBuildId string
}

func (r *BuildRegistry) ListBuilds(labels labels.Selector) (*api.BuildList, error) {
	return r.Builds, r.Err
}

func (r *BuildRegistry) GetBuild(id string) (*api.Build, error) {
	return r.Build, r.Err
}

func (r *BuildRegistry) CreateBuild(build *api.Build) error {
	return r.Err
}

func (r *BuildRegistry) UpdateBuild(build *api.Build) error {
	return r.Err
}

func (r *BuildRegistry) DeleteBuild(id string) error {
	r.DeletedBuildId = id
	return r.Err
}
