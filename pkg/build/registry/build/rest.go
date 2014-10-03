package build

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

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
func (r *REST) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	builds, err := r.registry.ListBuilds(selector)
	if err != nil {
		return nil, err
	}
	return builds, err

}

// Get obtains the build specified by its id.
func (r *REST) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	build, err := r.registry.GetBuild(id)
	if err != nil {
		return nil, err
	}
	return build, err
}

// Delete asynchronously deletes the Build specified by its id.
func (r *REST) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, r.registry.DeleteBuild(id)
	}), nil
}

// Create registers a given new Build instance to r.registry.
func (r *REST) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if len(build.ID) == 0 {
		build.ID = uuid.NewUUID().String()
	}
	if len(build.Status) == 0 {
		build.Status = api.BuildNew
	}
	build.CreationTimestamp = util.Now()
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.ID, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.CreateBuild(build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}

// Update replaces a given Build instance with an existing instance in r.registry.
func (r *REST) Update(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.ID, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.UpdateBuild(build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}
