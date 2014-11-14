package template

import (
	"errors"
	"fmt"
	"math/rand"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/latest"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/config"
	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	"github.com/openshift/origin/pkg/template/generator"
)

// REST implements RESTStorage interface for Template objects.
type REST struct{}

// NewREST creates new RESTStorage interface for Template objects.
func NewREST() *REST {
	return &REST{}
}

func (s *REST) New() runtime.Object {
	return &api.Template{}
}

func (s *REST) List(ctx kapi.Context, selector, fields labels.Selector) (runtime.Object, error) {
	return nil, errors.New("not implemented")
}

func (s *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return nil, errors.New("not implemented")
}

func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	// TODO: This should be MultiMapper to handle both Origin and K8s objects
	mapper := latest.RESTMapper
	typer := kapi.Scheme
	tpl, ok := obj.(*api.Template)
	if !ok {
		return nil, errors.New("Not a template config.")
	}
	if errs := validation.ValidateTemplate(mapper, typer, tpl); len(errs) > 0 {
		return nil, errors.New(fmt.Sprintf("Invalid template config: %#v", errs))
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		generators := map[string]generator.Generator{
			"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
		}
		processor := template.NewTemplateProcessor(generators)
		cfg, err := processor.Process(tpl)
		if err != nil {
			return nil, err
		}
		if err := config.AddConfigLabels(cfg, labels.Set{"template": tpl.Name}, mapper, typer); err != nil {
			return nil, err
		}
		return cfg, nil
	}), nil
}

func (s *REST) Update(ctx kapi.Context, tpl runtime.Object) (<-chan apiserver.RESTResult, error) {
	return nil, errors.New("not implemented")
}

func (s *REST) Delete(ctx kapi.Context, id string) (<-chan apiserver.RESTResult, error) {
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		return nil, errors.New("not implemented")
	}), nil
}
