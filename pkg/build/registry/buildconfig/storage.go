package buildconfig

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

// Storage is an implementation of RESTStorage for the api server.
type Storage struct {
	registry Registry
}

// NewStorage creates a new Storage for BuildConfig.
func NewStorage(registry Registry) apiserver.RESTStorage {
	return &Storage{
		registry: registry,
	}
}

// New creates a new BuildConfig.
func (storage *Storage) New() interface{} {
	return &api.BuildConfig{}
}

// List obtains a list of BuildConfigs that match selector.
func (storage *Storage) List(selector labels.Selector) (interface{}, error) {
	builds, err := storage.registry.ListBuildConfigs(selector)
	if err != nil {
		return nil, err
	}
	return builds, err
}

// Get obtains the BuildConfig specified by its id.
func (storage *Storage) Get(id string) (interface{}, error) {
	buildConfig, err := storage.registry.GetBuildConfig(id)
	if err != nil {
		return nil, err
	}
	return buildConfig, err
}

// Delete asynchronously deletes the BuildConfig specified by its id.
func (storage *Storage) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, storage.registry.DeleteBuildConfig(id)
	}), nil
}

// Extract deserializes user provided data into an api.BuildConfig.
func (storage *Storage) Extract(body []byte) (interface{}, error) {
	result := api.BuildConfig{}
	err := runtime.DecodeInto(body, &result)
	return result, err
}

// Create registers a given new BuildConfig instance to storage.registry.
func (storage *Storage) Create(obj interface{}) (<-chan interface{}, error) {
	buildConfig, ok := obj.(*api.BuildConfig)
	if !ok {
		return nil, fmt.Errorf("not a buildConfig: %#v", obj)
	}
	if len(buildConfig.ID) == 0 {
		buildConfig.ID = uuid.NewUUID().String()
	}
	buildConfig.CreationTimestamp = util.Now()
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, errors.NewInvalid("buildConfig", buildConfig.ID, errs)
	}
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := storage.registry.CreateBuildConfig(buildConfig)
		if err != nil {
			return nil, err
		}
		return buildConfig, nil
	}), nil
}

// Update replaces a given BuildConfig instance with an existing instance in storage.registry.
func (storage *Storage) Update(obj interface{}) (<-chan interface{}, error) {
	buildConfig, ok := obj.(*api.BuildConfig)
	if !ok {
		return nil, fmt.Errorf("not a buildConfig: %#v", obj)
	}
	if errs := validation.ValidateBuildConfig(buildConfig); len(errs) > 0 {
		return nil, errors.NewInvalid("buildConfig", buildConfig.ID, errs)
	}
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := storage.registry.UpdateBuildConfig(buildConfig)
		if err != nil {
			return nil, err
		}
		return buildConfig, nil
	}), nil
}
