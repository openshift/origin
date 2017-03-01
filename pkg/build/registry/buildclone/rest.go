package buildclone

import (
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/runtime"

	buildapi "github.com/openshift/origin/pkg/build/api"
	"github.com/openshift/origin/pkg/build/generator"
)

// NewREST creates a new storage object for build generation
func NewREST(generator *generator.BuildGenerator) *REST {
	return &REST{generator: generator}
}

// CloneREST is a RESTStorage implementation for a BuildGenerator which supports only
// the Get operation (as the generator has no underlying storage object).
type REST struct {
	generator *generator.BuildGenerator
}

// New creates a new build clone request
func (s *REST) New() runtime.Object {
	return &buildapi.BuildRequest{}
}

// Create instantiates a new build from an existing build
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	return s.generator.Clone(ctx, obj.(*buildapi.BuildRequest))
}
