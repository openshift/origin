package parameterizer

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"

	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	templatevalidation "github.com/openshift/origin/pkg/template/apis/template/validation"
	templateparameterizer "github.com/openshift/origin/pkg/template/parameterizer"
)

// REST implements RESTStorage interface for parameterizing Template or List objects.
type REST struct {
}

// NewREST creates new RESTStorage interface for parameterizing Template or List objects.
func NewREST() *REST {
	return &REST{}
}

// New returns a new Template
func (s *REST) New() runtime.Object {
	return &templateapi.ParameterizeTemplateRequest{}
}

var (
	parameterizers = map[templateapi.ParameterizableAspect]templateparameterizer.Parameterizer{
		templateapi.ImageReferencesAspect: templateparameterizer.ImageRefParameterizer,
	}
	defaultParameterizers = []templateparameterizer.Parameterizer{templateparameterizer.ImageRefParameterizer}
)

const (
	parameterizerAnnotation = "template.openshift.io/parameterizers"
)

// Create processes a Template and creates a new list of objects
func (s *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	request, ok := obj.(*templateapi.ParameterizeTemplateRequest)
	if !ok {
		return nil, errors.NewBadRequest("not a parameterize request")
	}

	// determine which parameterizers to use
	whichParameterizers := defaultParameterizers
	if len(request.Aspects) > 0 {
		whichParameterizers = []templateparameterizer.Parameterizer{}
		for _, aspect := range request.Aspects {
			parameterizer, ok := parameterizers[aspect]
			if !ok {
				return nil, errors.NewBadRequest(fmt.Sprintf("invalid parameterizable aspect: %s", aspect))
			}
			whichParameterizers = append(whichParameterizers, parameterizer)
		}
	}

	if errs := templatevalidation.ValidateParameterizeTemplateRequest(request); len(errs) > 0 {
		return nil, errors.NewInvalid(templateapi.Kind("ParameterizeTemplateRequest"), "", errs)
	}

	errs := templateparameterizer.Parameterize(&request.Template, whichParameterizers)
	if len(errs) > 0 {
		return nil, utilerrors.NewAggregate(errs)
	}

	for i := range request.Template.Objects {
		request.Template.Objects[i] = runtime.NewEncodable(unstructured.UnstructuredJSONScheme, request.Template.Objects[i])
	}

	return &request.Template, nil
}
