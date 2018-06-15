package internalversion

import (
	templateapi "github.com/openshift/origin/pkg/template/apis/template"
	"k8s.io/apimachinery/pkg/api/errors"
)

// TemplateListerExpansion allows custom methods to be added to
// TemplateLister.
type TemplateListerExpansion interface {
	GetByUID(uid string) (*templateapi.Template, error)
}

// TemplateNamespaceListerExpansion allows custom methods to be added to
// TemplateNamespaceLister.
type TemplateNamespaceListerExpansion interface{}

func (s templateLister) GetByUID(uid string) (*templateapi.Template, error) {
	templates, err := s.indexer.ByIndex(templateapi.TemplateUIDIndex, uid)
	if err != nil {
		return nil, err
	}
	if len(templates) == 0 {
		return nil, errors.NewNotFound(templateapi.Resource("template"), uid)
	}
	return templates[0].(*templateapi.Template), nil
}
