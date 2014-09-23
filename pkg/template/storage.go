package template

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	. "github.com/openshift/origin/pkg/template/generator"
)

// Storage implements RESTStorage for the Template objects.
type Storage struct{}

// NewStorage creates new RESTStorage for the Template objects.
func NewStorage() apiserver.RESTStorage {
	return &Storage{}
}

func (s *Storage) List(selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.New("template.Storage.List() is not implemented.")
}

func (s *Storage) Get(id string) (runtime.Object, error) {
	return nil, errors.New("template.Storage.Get() is not implemented.")
}

func (s *Storage) New() runtime.Object {
	return &api.Template{}
}

func (s *Storage) Delete(id string) (<-chan runtime.Object, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return nil, errors.New("template.Storage.Delete() is not implemented.")
	}), nil
}

func (s *Storage) Update(minion runtime.Object) (<-chan runtime.Object, error) {
	return nil, errors.New("template.Storage.Update() is not implemented.")
}

func (s *Storage) Create(obj runtime.Object) (<-chan runtime.Object, error) {
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
		config, err := processor.Process(template)
		return config, err
	}), nil
}
