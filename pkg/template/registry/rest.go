package template

import (
	"errors"
	"math/rand"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"

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
	tpl, ok := obj.(*api.Template)
	if !ok {
		return nil, apierr.NewBadRequest("not a template")
	}
	if len(tpl.Name) == 0 {
		tpl.Name = "default"
	}
	kapi.FillObjectMetaSystemFields(ctx, &tpl.ObjectMeta)
	if errs := validation.ValidateTemplate(tpl); len(errs) > 0 {
		return nil, apierr.NewInvalid("template", tpl.Name, errs)
	}
	return apiserver.MakeAsync(func() (runtime.Object, error) {
		generators := map[string]generator.Generator{
			"expression": generator.NewExpressionValueGenerator(rand.New(rand.NewSource(time.Now().UnixNano()))),
		}
		processor := template.NewProcessor(generators)
		cfg, err := processor.Process(tpl)
		if len(err) > 0 {
			// TODO: We don't report the processing errors to users as there is no
			// good way how to do it for just some items.
			glog.V(1).Infof(util.SliceToError(err).Error())
		}

		if err := config.AddConfigLabels(cfg, labels.Set{"template": tpl.Name}); len(err) > 0 {
			// TODO: We don't report the processing errors to users as there is no
			// good way how to do it for just some items.
			glog.V(1).Infof(util.SliceToError(err).Error())
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
