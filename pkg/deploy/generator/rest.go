package generator

import (
	"errors"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

type REST struct {
	generator *DeploymentConfigGenerator
	codec     runtime.Codec
}

func NewREST(generator *DeploymentConfigGenerator, codec runtime.Codec) apiserver.RESTStorage {
	return &REST{generator: generator, codec: codec}
}

func (s *REST) New() runtime.Object {
	return &deployapi.DeploymentConfig{}
}

func (s *REST) List(ctx api.Context, labels, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.New("deploy/generator.REST.List() is not implemented.")
}

func (s *REST) Get(ctx api.Context, id string) (runtime.Object, error) {
	return s.generator.Generate(id)
}

func (s *REST) Delete(ctx api.Context, id string) (<-chan runtime.Object, error) {
	return nil, errors.New("deploy/generator.REST.Delete() is not implemented.")
}

func (s *REST) Update(ctx api.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, errors.New("deploy/generator.REST.Update() is not implemented.")
}

func (s *REST) Create(ctx api.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	return nil, errors.New("deploy/generator.REST.Create() is not implemented.")
}
