package generator

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/api/validation"
)

// NewREST creates a new storage object for build generation
func NewREST(generator *BuildGenerator) (*CloneREST, *InstantiateREST) {
	return &CloneREST{generator: generator}, &InstantiateREST{generator: generator}
}

// CloneREST is a RESTStorage implementation for a BuildGenerator which supports only
// the Get operation (as the generator has no underlying storage object).
type CloneREST struct {
	generator *BuildGenerator
}

// New creates a new build clone request
func (s *CloneREST) New() runtime.Object {
	return &buildapi.BuildRequest{}
}

// Create instantiates a new build from an existing build
func (s *CloneREST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	request, ok := obj.(*buildapi.BuildRequest)
	if !ok {
		return nil, fmt.Errorf("not a buildRequest: %#v", obj)
	}
	if errs := validation.ValidateBuildRequest(request); len(errs) > 0 {
		return nil, errors.NewInvalid("buildRequest", request.Name, errs)
	}
	return s.generator.Clone(ctx, request)
}

// InstantiateREST is a RESTStorage implementation for a BuildGenerator which supports only
// the Get operation (as the generator has no underlying storage object).
type InstantiateREST struct {
	generator *BuildGenerator
}

// New creates a new build generation request
func (s *InstantiateREST) New() runtime.Object {
	return &buildapi.BuildRequest{}
}

// Create instantiates a new build from a build configuration
func (s *InstantiateREST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	request, ok := obj.(*buildapi.BuildRequest)
	if !ok {
		return nil, fmt.Errorf("not a buildRequest: %#v", obj)
	}
	if errs := validation.ValidateBuildRequest(request); len(errs) > 0 {
		return nil, errors.NewInvalid("buildRequest", request.Name, errs)
	}
	return s.generator.Instantiate(ctx, request)
}
