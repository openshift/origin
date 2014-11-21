package build

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for builds.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New creates a new Build object
func (r *REST) New() runtime.Object {
	return &api.Build{}
}

// List obtains a list of Builds that match selector.
func (r *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	builds, err := r.registry.ListBuilds(ctx, selector)
	if err != nil {
		return nil, err
	}
	return builds, err

}

// Get obtains the build specified by its id.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	build, err := r.registry.GetBuild(ctx, id)
	if err != nil {
		return nil, err
	}
	return build, err
}

// Delete asynchronously deletes the Build specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteBuild(ctx, id)
	}), nil
}

// Create registers a given new Build instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict("build", build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}

	if len(build.Name) == 0 {
		build.Name = uuid.NewUUID().String()
	}
	if len(build.Status) == 0 {
		build.Status = api.BuildStatusNew
	}
	build.CreationTimestamp = util.Now()
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.Name, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.CreateBuild(ctx, build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}

// Update replaces a given Build instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.Name, errs)
	}
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict("build", build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.UpdateBuild(ctx, build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}

// Watch begins watching for new, changed, or deleted Builds.
func (s *REST) Watch(ctx kapi.Context, label, field labels.Selector, resourceVersion string) (watch.Interface, error) {
	return s.registry.WatchBuilds(ctx, label, field, resourceVersion)
}
