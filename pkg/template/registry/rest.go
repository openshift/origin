package registry

import (
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/registry/generic"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	utilerr "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
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

// ResetBeforeCreate clears fields that are not allowed to be set by end users on creation.
func (templateStrategy) ResetBeforeCreate(obj runtime.Object) {
}

// Validate validates a new template.
func (templateStrategy) Validate(obj runtime.Object) errors.ValidationErrorList {
	template := obj.(*api.Template)
	return validation.ValidateTemplate(template)
}

// AllowCreateOnUpdate is false for templates.
func (templateStrategy) AllowCreateOnUpdate() bool {
	return false
}

// ValidateUpdate is the default update validation for an end user.
func (templateStrategy) ValidateUpdate(obj, old runtime.Object) errors.ValidationErrorList {
	return validation.ValidateTemplateUpdate(obj.(*api.Template), old.(*api.Template))
}

// MatchTemplate returns a generic matcher for a given label and field selector.
func MatchTemplate(label, field labels.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		o, ok := obj.(*api.Template)
		if !ok {
			return false, fmt.Errorf("not a pod")
		}
		return label.Matches(labels.Set(o.Labels)), nil
	})
}

// REST implements RESTStorage interface for processing Template objects.
type REST struct{}

// NewREST creates new RESTStorage interface for processing Template objects.
func NewREST() *REST {
	return &REST{}
}

func (s *REST) New() runtime.Object {
	return &api.Template{}
}

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
	cfg, err := processor.Process(tpl)
	if len(err) > 0 {
		// TODO: We don't report the processing errors to users as there is no
		// good way how to do it for just some items.
		glog.V(1).Infof(utilerr.NewAggregate(err).Error())
	}

	if tpl.ObjectLabels != nil {
		objectLabels := labels.Set(tpl.ObjectLabels)
		if err := template.AddConfigLabels(cfg, objectLabels); len(err) > 0 {
			// TODO: We don't report the processing errors to users as there is no
			// good way how to do it for just some items.
			glog.V(1).Infof(utilerr.NewAggregate(err).Error())
		}
	}
	return cfg, nil
}

// retrieveFunc is a function that fetches the contents of a given URL

type fetcher interface {
	Fetch(URL string) ([]byte, error)
}

// RemoteREST implements RESTStorage interface for fetching remote Templates
type RemoteREST struct {
	codec   runtime.Codec
	fetcher fetcher
}

// NewRemoteREST creates a new RESTStorage interface for fetching remote Template objects
func NewRemoteREST(codec runtime.Codec) *RemoteREST {
	remoteREST := &RemoteREST{
		codec: codec,
	}
	remoteREST.fetcher = remoteREST
	return remoteREST
}

// New creates a new empty remote template
func (r *RemoteREST) New() runtime.Object {
	return &api.RemoteTemplate{}
}

// Create fetches a remote template and ensures it is a valid template
func (r *RemoteREST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	remote, ok := obj.(*api.RemoteTemplate)
	if !ok {
		return nil, errors.NewBadRequest("not a remote template")
	}
	body, err := r.fetcher.Fetch(remote.RemoteURL)
	if err != nil {
		return nil, err
	}
	templateObject, err := r.codec.Decode(body)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("error decoding template from %s: %v",
			remote.RemoteURL, err))
	}
	template, ok := templateObject.(*api.Template)
	if !ok {
		return nil, errors.NewInternalError(fmt.Errorf("resource at %s is not a template."))
	}
	if errs := validation.ValidateProcessedTemplate(template); len(errs) > 0 {
		return nil, errors.NewInvalid("template", template.Name, errs)
	}
	return template, nil
}

func (r *RemoteREST) Fetch(remoteURL string) ([]byte, error) {
	resp, err := http.Get(remoteURL)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("error retrieving template from %s: %v",
			remoteURL, err))
	}
	defer resp.Body.Close()
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, errors.NewInternalError(fmt.Errorf("error reading data from %s: %v",
			remoteURL, err))
	}
	return body, nil
}
