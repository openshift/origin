package template

import (
	"math/rand"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	"github.com/openshift/origin/pkg/template"
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
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
	return &templateapi.Template{}
}

// Create processes a Template and creates a new list of objects
func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	tpl, ok := obj.(*templateapi.Template)
	if !ok {
		return nil, errors.NewBadRequest("not a template")
	}
	if errs := templatevalidation.ValidateProcessedTemplate(tpl); len(errs) > 0 {
		return nil, errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	}
	processor := template.NewProcessor(generators)
	if errs := processor.Process(tpl); len(errs) > 0 {
		glog.V(1).Infof(errs.ToAggregate().Error())
		return nil, errors.NewInvalid(templateapi.Kind("Template"), tpl.Name, errs)
	}

	// we know that we get back runtime.Unstructured objects from the Process call.  We need to encode those
	// objects using the unstructured codec BEFORE the REST layers gets its shot at encoding to avoid a layered
	// encode being done.
	for i := range tpl.Objects {
		tpl.Objects[i] = runtime.NewEncodable(unstructured.UnstructuredJSONScheme, tpl.Objects[i])
	}

	return tpl, nil
}
