package generator

import (
	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// REST is a RESTStorage implementation for a DeploymentConfigGenerator which supports only
// the Get operation (as the generator has no underlying storage object).
type REST struct {
	generator *DeploymentConfigGenerator
	codec     runtime.Codec
}

func NewREST(generator *DeploymentConfigGenerator, codec runtime.Codec) *REST {
	return &REST{generator: generator, codec: codec}
}

func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentConfig{}
}

func (s *REST) Get(ctx api.Context, id string) (runtime.Object, error) {
	return s.generator.Generate(ctx, id)
}
