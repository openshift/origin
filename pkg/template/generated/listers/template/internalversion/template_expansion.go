package internalversion

import (
	"github.com/openshift/origin/pkg/template/api"
	"k8s.io/apimachinery/pkg/api/errors"
)

// TemplateListerExpansion allows custom methods to be added to
// TemplateLister.
type TemplateListerExpansion interface {
	GetByUID(uid string) (*api.Template, error)
}

// TemplateNamespaceListerExpansion allows custom methods to be added to
// TemplateNamespaceLister.
type TemplateNamespaceListerExpansion interface{}

func (s templateLister) GetByUID(uid string) (*api.Template, error) {
	templates, err := s.indexer.ByIndex(api.TemplateUIDIndex, uid)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, errors.NewNotFound(api.Resource("template"), uid)
	}
	return templates[0].(*api.Template), nil
}
