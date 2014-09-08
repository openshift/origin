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

// Storage is an implementation of RESTStorage for the api server.
type Storage struct {
	registry Registry
}

// NewStorage creates a new Storage for builds.
func NewStorage(registry Registry) apiserver.RESTStorage {
	return &Storage{
		registry: registry,
	}
}

// New creates a new Build object
func (storage *Storage) New() interface{} {
	return &api.Build{}
}

// List obtains a list of Builds that match selector.
func (storage *Storage) List(selector labels.Selector) (interface{}, error) {
	builds, err := storage.registry.ListBuilds(selector)
	if err != nil {
		return nil, err
	}
	return builds, err

}

// Get obtains the build specified by its id.
func (storage *Storage) Get(id string) (interface{}, error) {
	build, err := storage.registry.GetBuild(id)
	if err != nil {
		return nil, err
	}
	return build, err
}

// Delete asynchronously deletes the Build specified by its id.
func (storage *Storage) Delete(id string) (<-chan interface{}, error) {
	return apiserver.MakeAsync(func() (interface{}, error) {
		return &kubeapi.Status{Status: kubeapi.StatusSuccess}, storage.registry.DeleteBuild(id)
	}), nil
}

// Extract deserializes user provided data into an api.Build.
func (storage *Storage) Extract(body []byte) (interface{}, error) {
	result := api.Build{}
	err := runtime.DecodeInto(body, &result)
	return result, err
}

// Create registers a given new Build instance to storage.registry.
func (storage *Storage) Create(obj interface{}) (<-chan interface{}, error) {
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
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := storage.registry.CreateBuild(build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}

// Update replaces a given Build instance with an existing instance in storage.registry.
func (storage *Storage) Update(obj interface{}) (<-chan interface{}, error) {
	build, ok := obj.(*api.Build)
	if !ok {
		return nil, fmt.Errorf("not a build: %#v", obj)
	}
	if errs := validation.ValidateBuild(build); len(errs) > 0 {
		return nil, errors.NewInvalid("build", build.ID, errs)
	}
	return apiserver.MakeAsync(func() (interface{}, error) {
		err := storage.registry.UpdateBuild(build)
		if err != nil {
			return nil, err
		}
		return build, nil
	}), nil
}
