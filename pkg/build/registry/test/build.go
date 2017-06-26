package test

import (
	"sync"

	metainternal "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
)

type BuildRegistry struct {
	Err            error
	Builds         *buildapi.BuildList
	Build          *buildapi.Build
	DeletedBuildID string
	sync.Mutex
}

func (r *BuildRegistry) ListBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (*buildapi.BuildList, error) {
	r.Lock()
	defer r.Unlock()
	return r.Builds, r.Err
}

func (r *BuildRegistry) GetBuild(ctx apirequest.Context, id string) (*buildapi.Build, error) {
	r.Lock()
	defer r.Unlock()
	return r.Build, r.Err
}

func (r *BuildRegistry) CreateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	r.Lock()
	defer r.Unlock()
	r.Build = build
	return r.Err
}

func (r *BuildRegistry) UpdateBuild(ctx apirequest.Context, build *buildapi.Build) error {
	r.Lock()
	defer r.Unlock()
	r.Build = build
	return r.Err
}

func (r *BuildRegistry) DeleteBuild(ctx apirequest.Context, id string) error {
	r.Lock()
	defer r.Unlock()
	r.DeletedBuildID = id
	r.Build = nil
	return r.Err
}

func (r *BuildRegistry) WatchBuilds(ctx apirequest.Context, options *metainternal.ListOptions) (watch.Interface, error) {
	return nil, r.Err
}

type BuildStorage struct {
	Err    error
	Build  *buildapi.Build
	Builds *buildapi.BuildList
	sync.Mutex
}

func (r *BuildStorage) Get(ctx apirequest.Context, id string, options *metav1.GetOptions) (runtime.Object, error) {
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

func (r *BuildStorageWithOptions) Get(ctx apirequest.Context, id string, opts runtime.Object) (runtime.Object, error) {
	r.Lock()
	defer r.Unlock()
	return r.Build, r.Err
}
