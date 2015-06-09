package registry

import (
	"math/rand"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	utilerr "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
	templatevalidation "github.com/openshift/origin/pkg/template/api/validation"
	"github.com/openshift/origin/pkg/template/generator"
)

// REST implements RESTStorage interface for processing Template objects.
type REST struct {
}

// NewREST creates new RESTStorage interface for processing Template objects. If
// legacyReturn is used, a Config object is returned. Otherwise, a List is returned
func NewREST() *REST {
	return &REST{}
}

// New returns a new Template
// TODO: this is the input, but not the output. pkg/api/rest should probably allow
// a rest.Storage object to vary its output or input types (not sure whether New()
// should be input or output... probably input).
func (s *REST) New() runtime.Object {
	return &api.Template{}
}

// Create processes a Template and creates a new list of objects
func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	tpl, ok := obj.(*api.Template)
	if !ok {
		return nil, errors.NewBadRequest("not a template")
	}
	if errs := templatevalidation.ValidateProcessedTemplate(tpl); len(errs) > 0 {
		return nil, errors.NewInvalid("template", tpl.Name, errs)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	}
	processor := template.NewProcessor(generators)
	if errs := processor.Process(tpl); len(errs) > 0 {
		glog.V(1).Infof(utilerr.NewAggregate(errs).Error())
		return nil, errors.NewInvalid("template", tpl.Name, errs)
	}

	return tpl, nil
}
