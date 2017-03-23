package cache

import (
	templateapi "github.com/openshift/origin/pkg/template/api"
	"k8s.io/kubernetes/pkg/client/cache"
)

type StoreToTemplateLister interface {
	List() ([]*templateapi.Template, error)
	GetTemplateByUID(uid string) (*templateapi.Template, error)
}

type StoreToTemplateListerImpl struct {
	cache.Indexer
}

func (s *StoreToTemplateListerImpl) List() ([]*templateapi.Template, error) {
	list := s.Indexer.List()

	templates := make([]*templateapi.Template, len(list))
	for i, template := range list {
		templates[i] = template.(*templateapi.Template)
	}
	return templates, nil
}

func (s *StoreToTemplateListerImpl) GetTemplateByUID(uid string) (*templateapi.Template, error) {
	templates, err := s.Indexer.ByIndex(TemplateUIDIndex, uid)
	if err != nil || len(templates) == 0 {
		return nil, err
	}
	return templates[0].(*templateapi.Template), nil
}
