package buildconfig

import (
	"fmt"

	"code.google.com/p/go-uuid/uuid"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	"github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// REST is an implementation of RESTStorage for the api server.
type REST struct {
	registry Registry
}

// NewREST creates a new REST for BuildConfig.
func NewREST(registry Registry) apiserver.RESTStorage {
	return &REST{registry}
}

// New creates a new BuildConfig.
func (r *REST) New() runtime.Object {
	return &api.BuildConfig{}
}

// List obtains a list of BuildConfigs that match selector.
func (r *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	builds, err := r.registry.ListBuildConfigs(ctx, selector)
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
func (r *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return &kapi.Status{Status: kapi.StatusSuccess}, r.registry.DeleteBuildConfig(ctx, id)
	}), nil
}

// Create registers a given new BuildConfig instance to r.registry.
func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
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
	buildConfig.CreationTimestamp = util.Now()
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, errors.NewInvalid("buildConfig", buildConfig.Name, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.CreateBuildConfig(ctx, buildConfig)
		if err != nil {
			return nil, err
		}
		return buildConfig, nil
	}), nil
}

// Update replaces a given BuildConfig instance with an existing instance in r.registry.
func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	buildConfig, ok := obj.(*api.BuildConfig)
	if !ok {
		return nil, fmt.Errorf("not a buildConfig: %#v", obj)
	}
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, errors.NewInvalid("buildConfig", buildConfig.Name, errs)
	}
	if !kapi.ValidNamespace(ctx, &buildConfig.ObjectMeta) {
		return nil, errors.NewConflict("buildConfig", buildConfig.Namespace, fmt.Errorf("BuildConfig.Namespace does not match the provided context"))
	}

	return apiserver.MakeAsync(func() (runtime.Object, error) {
		err := r.registry.UpdateBuildConfig(ctx, buildConfig)
		if err != nil {
			return nil, err
		}
		return buildConfig, nil
	}), nil
}
