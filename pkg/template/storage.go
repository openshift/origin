package template

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	kubeapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/config"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	. "github.com/openshift/origin/pkg/template/generator"
)

// Storage implements RESTStorage for the Template objects.
type Storage struct{}

// NewStorage creates new RESTStorage for the Template objects.
func NewStorage() *Storage {
	return &Storage{}
}

func (s *Storage) New() runtime.Object {
	return &api.Template{}
}

func (s *Storage) List(ctx kubeapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.New("template.Storage.List() is not implemented.")
}

func (s *Storage) Get(ctx kubeapi.Context, id string) (runtime.Object, error) {
	return nil, errors.New("template.Storage.Get() is not implemented.")
}

func (s *Storage) Create(ctx kubeapi.Context, obj runtime.Object) (<-chan runtime.Object, error) {
	template, ok := obj.(*api.Template)
	if !ok {
		return nil, errors.New("Not a template config.")
	}
	if errs := validation.ValidateTemplate(template); len(errs) > 0 {
		return nil, errors.New(fmt.Sprintf("Invalid template config: %#v", errs))
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		generators := map[string]Generator{
			"expression": NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
		}
		processor := NewTemplateProcessor(generators)
		cfg, err := processor.Process(template)
		if err != nil {
			return nil, err
		}
		if err := config.AddConfigLabels(cfg, labels.Set{"template": template.ID}); err != nil {
			return nil, err
		}
		return cfg, nil
	}), nil
}

func (s *Storage) Update(ctx kubeapi.Context, template runtime.Object) (<-chan runtime.Object, error) {
	return nil, errors.New("template.Storage.Update() is not implemented.")
}

func (s *Storage) Delete(ctx kubeapi.Context, id string) (<-chan runtime.Object, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return nil, errors.New("template.Storage.Delete() is not implemented.")
	}), nil
}
