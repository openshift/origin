package template

import (
	"math/rand"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	apierr "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/apiserver"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	utilerr "github.com/GoogleCloudPlatform/kubernetes/pkg/util/errors"
	"github.com/golang/glog"
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

func (s *REST) Create(ctx kapi.Context, obj runtime.Object) (<-chan apiserver.RESTResult, error) {
	tpl, ok := obj.(*api.Template)
	if !ok {
		return nil, apierr.NewBadRequest("not a template")
	}
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
			glog.V(1).Infof(utilerr.NewAggregate(err).Error())
		}

		if err := template.AddConfigLabels(cfg, labels.Set{"template": tpl.Name}); len(err) > 0 {
			// TODO: We don't report the processing errors to users as there is no
			// good way how to do it for just some items.
			glog.V(1).Infof(utilerr.NewAggregate(err).Error())
		}
		return cfg, nil
	}), nil
}
