package test

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type BuildRegistry struct {
	Err            error
	Builds         *buildapi.BuildList
	Build          *buildapi.Build
	DeletedBuildId string
}

func (r *BuildRegistry) ListBuilds(ctx kapi.Context, labels labels.Selector) (*buildapi.BuildList, error) {
	return r.Builds, r.Err
}

func (r *BuildRegistry) GetBuild(ctx kapi.Context, id string) (*buildapi.Build, error) {
	return r.Build, r.Err
}

func (r *BuildRegistry) CreateBuild(ctx kapi.Context, build *buildapi.Build) error {
	return r.Err
}

func (r *BuildRegistry) UpdateBuild(ctx kapi.Context, build *buildapi.Build) error {
	return r.Err
}

func (r *BuildRegistry) DeleteBuild(ctx kapi.Context, id string) error {
	r.DeletedBuildId = id
	return r.Err
}

func (r *BuildRegistry) WatchBuilds(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return nil, r.Err
}
