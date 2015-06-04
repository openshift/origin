package registry

import (
	"fmt"
	"math/rand"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/fields"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	utilerr "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/template"
	"github.com/openshift/origin/pkg/template/api"
	"github.com/openshift/origin/pkg/template/api/validation"
	"github.com/openshift/origin/pkg/template/generator"
)

// templateStrategy implements behavior for Templates
type templateStrategy struct {
	runtime.ObjectTyper
	kapi.NameGenerator
}

// Strategy is the default logic that applies when creating and updating Template
// objects via the REST API.
var Strategy = templateStrategy{kapi.Scheme, kapi.SimpleNameGenerator}

// NamespaceScoped is true for templates.
func (templateStrategy) NamespaceScoped() bool {
	return true
}

// PrepareForUpdate clears fields that are not allowed to be set by end users on update.
func (templateStrategy) PrepareForUpdate(obj, old runtime.Object) {}

// PrepareForCreate clears fields that are not allowed to be set by end users on creation.
func (templateStrategy) PrepareForCreate(obj runtime.Object) {
}

// Validate validates a new template.
func (templateStrategy) Validate(ctx kapi.Context, obj runtime.Object) fielderrors.ValidationErrorList {
	template := obj.(*api.Template)
	return validation.ValidateTemplate(template)
}

// AllowCreateOnUpdate is false for templates.
func (templateStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (templateStrategy) ValidateUpdate(ctx kapi.Context, obj, old runtime.Object) fielderrors.ValidationErrorList {
	return validation.ValidateTemplateUpdate(obj.(*api.Template), old.(*api.Template))
}

// MatchTemplate returns a generic matcher for a given label and field selector.
func MatchTemplate(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		o, ok := obj.(*api.Template)
		if !ok {
			return false, fmt.Errorf("not a pod")
		}
		return label.Matches(labels.Set(o.Labels)), nil
	})
}

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
	if errs := validation.ValidateProcessedTemplate(tpl); len(errs) > 0 {
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
