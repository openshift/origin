package registry

import (
	"math/rand"
	"time"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"

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
		return nil, errors.NewInvalid(api.Kind("Template"), tpl.Name, errs)
	}

	generators := map[string]generator.Generator{
		"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
	}
	processor := template.NewProcessor(generators)
	if errs := processor.Process(tpl); len(errs) > 0 {
		glog.V(1).Infof(errs.ToAggregate().Error())
		return nil, errors.NewInvalid(api.Kind("Template"), tpl.Name, errs)
	}

	// we know that we get back runtime.Unstructured objects from the Process call.  We need to encode those
	// objects using the unstructured codec BEFORE the REST layers gets its shot at encoding to avoid a layered
	// encode being done.
	for i := range tpl.Objects {
		tpl.Objects[i] = runtime.NewEncodable(runtime.UnstructuredJSONScheme, tpl.Objects[i])
	}

	return tpl, nil
}
