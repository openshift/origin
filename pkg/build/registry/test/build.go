package test

import (
	"sync"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/watch"

	buildapi "github.com/openshift/origin/pkg/build/api"
)

type BuildRegistry struct {
	Err            error
	Builds         *buildapi.BuildList
	Build          *buildapi.Build
	DeletedBuildID string
	sync.Mutex
}

func (r *BuildRegistry) ListBuilds(ctx kapi.Context, options *kapi.ListOptions) (*buildapi.BuildList, error) {
	r.Lock()
	defer r.Unlock()
	return r.Builds, r.Err
}

func (r *BuildRegistry) GetBuild(ctx kapi.Context, id string) (*buildapi.Build, error) {
	r.Lock()
	defer r.Unlock()
	return r.Build, r.Err
}

func (r *BuildRegistry) CreateBuild(ctx kapi.Context, build *buildapi.Build) error {
	r.Lock()
	defer r.Unlock()
	r.Build = build
	return r.Err
}

func (r *BuildRegistry) UpdateBuild(ctx kapi.Context, build *buildapi.Build) error {
	r.Lock()
	defer r.Unlock()
	r.Build = build
	return r.Err
}

func (r *BuildRegistry) DeleteBuild(ctx kapi.Context, id string) error {
	r.Lock()
	defer r.Unlock()
	r.DeletedBuildID = id
	r.Build = nil
	return r.Err
}

func (r *BuildRegistry) WatchBuilds(ctx kapi.Context, options *kapi.ListOptions) (watch.Interface, error) {
	return nil, r.Err
}

type BuildStorage struct {
	Err    error
	Build  *buildapi.Build
	Builds *buildapi.BuildList
	sync.Mutex
}

func (r *BuildStorage) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	r.Lock()
	defer r.Unlock()
	// TODO: Use the list of builds in all of the rest registry calls
	if r.Builds != nil {
		for _, build := range r.Builds.Items {
			if build.Name == id {
				return &build, r.Err
			}
		}
	}
	return r.Build, r.Err
}

type BuildStorageWithOptions struct {
	Err   error
	Build *buildapi.Build
	sync.Mutex
}

func (r *BuildStorageWithOptions) NewGetOptions() (runtime.Object, bool, string) {
	return nil, false, ""
}

func (r *BuildStorageWithOptions) Get(ctx kapi.Context, id string, opts runtime.Object) (runtime.Object, error) {
	r.Lock()
	defer r.Unlock()
	return r.Build, r.Err
}
