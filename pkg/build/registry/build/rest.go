package build

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/watch"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// REST implements the RESTStorage interface in terms of an Registry.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for builds.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New creates a new Build object
func (r *REST) New() runtime.Object {
	return &api.Build{}
}

func (*REST) NewList() runtime.Object {
	return &api.Build{}
}

// List obtains a list of Builds that match label.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	builds, err := r.registry.ListBuilds(ctx, label)
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
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteBuild(ctx, id)
}

// Create registers a given new Build instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, errors.NewConflict("build", build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}

	if len(build.Name) == 0 {
		if len(build.Labels[api.BuildConfigLabel]) != 0 {
			build.Name = fmt.Sprintf("%s-%s", build.Labels[api.BuildConfigLabel], uuid.NewUUID().String())
		} else {
			build.Name = uuid.NewUUID().String()
		}
	}

	if len(build.Status) == 0 {
		build.Status = api.BuildStatusNew
	}
	kapi.FillObjectMetaSystemFields(ctx, &build.ObjectMeta)
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.Name, errs)
	}
	err := r.registry.CreateBuild(ctx, build)
	if err != nil {
		return nil, err
	}
	return build, nil
}

// Update replaces a given Build instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, false, fmt.Errorf("not a build: %#v", obj)
	}
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, false, errors.NewInvalid("build", build.Name, errs)
	}
	if !kapi.ValidNamespace(ctx, &build.ObjectMeta) {
		return nil, false, errors.NewConflict("build", build.Namespace, fmt.Errorf("Build.Namespace does not match the provided context"))
	}

	err := r.registry.UpdateBuild(ctx, build)
	if err != nil {
		return nil, false, err
	}
	out, err := r.Get(ctx, build.Name)
	return out, false, err
}

// Watch begins watching for new, changed, or deleted Builds.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.registry.WatchBuilds(ctx, label, field, resourceVersion)
}
