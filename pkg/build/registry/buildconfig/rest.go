package buildconfig

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

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for BuildConfig.
func NewREST(registry Registry) *REST {
	return &REST{registry}
}

// New creates a new BuildConfig.
func (r *REST) New() runtime.Object {
	return &api.BuildConfig{}
}

func (*REST) NewList() runtime.Object {
	return &api.BuildConfig{}
}

// List obtains a list of BuildConfigs that match label.
func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	builds, err := r.registry.ListBuildConfigs(ctx, label)
	if err != nil {
		return nil, err
	}
	return builds, err
}

// Get obtains the BuildConfig specified by its id.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	buildConfig, err := r.registry.GetBuildConfig(ctx, id)
	if err != nil {
		return nil, err
	}
	return buildConfig, err
}

// Delete asynchronously deletes the BuildConfig specified by its id.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteBuildConfig(ctx, id)
}

// Create registers a given new BuildConfig instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	buildConfig, ok := obj.(*api.BuildConfig)
	if !ok {
		return nil, fmt.Errorf("not a buildConfig: %#v", obj)
	}
	if !kapi.ValidNamespace(ctx, &buildConfig.ObjectMeta) {
		return nil, errors.NewConflict("buildConfig", buildConfig.Namespace, fmt.Errorf("BuildConfig.Namespace does not match the provided context"))
	}

	if len(buildConfig.Name) == 0 {
		buildConfig.Name = uuid.NewUUID().String()
	}
	kapi.FillObjectMetaSystemFields(ctx, &buildConfig.ObjectMeta)
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, errors.NewInvalid("buildConfig", buildConfig.Name, errs)
	}
	err := r.registry.CreateBuildConfig(ctx, buildConfig)
	if err != nil {
		return nil, err
	}
	return buildConfig, nil
}

// Update replaces a given BuildConfig instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	buildConfig, ok := obj.(*api.BuildConfig)
	if !ok {
		return nil, false, fmt.Errorf("not a buildConfig: %#v", obj)
	}
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, false, errors.NewInvalid("buildConfig", buildConfig.Name, errs)
	}
	if !kapi.ValidNamespace(ctx, &buildConfig.ObjectMeta) {
		return nil, false, errors.NewConflict("buildConfig", buildConfig.Namespace, fmt.Errorf("BuildConfig.Namespace does not match the provided context"))
	}

	err := r.registry.UpdateBuildConfig(ctx, buildConfig)
	if err != nil {
		return nil, false, err
	}
	out, err := r.Get(ctx, buildConfig.Name)
	return out, false, err
}

// Watch begins watching for new, changed, or deleted BuildConfigs.
func (r *REST) Watch(ctx kapi.Context, label labels.Selector, field fields.Selector, resourceVersion string) (watch.Interface, error) {
	return r.registry.WatchBuildConfigs(ctx, label, field, resourceVersion)
}
