package app

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"
	"github.com/openshift/origin/pkg/client"
)

type TemplateResolver struct {
	Client                    client.TemplatesNamespacer
	TemplateConfigsNamespacer client.TemplateConfigsNamespacer
	Namespaces                []string
}

func (r TemplateResolver) Resolve(value string) (*ComponentMatch, error) {
	checked := util.NewStringSet()

	for _, namespace := range r.Namespaces {
		if checked.Has(namespace) {
			continue
		}

		checked.Insert(namespace)

		glog.V(4).Infof("checking template %s/%s", namespace, value)
		repo, err := r.Client.Templates(namespace).Get(value)
		if err != nil {
			if errors.IsNotFound(err) || errors.IsForbidden(err) {
				continue
			}
			return nil, err
		}

		return &ComponentMatch{
			Value:       value,
			Argument:    fmt.Sprintf("--template=%q", value),
			Name:        value,
			Description: fmt.Sprintf("Template %s in project %s", repo.Name, repo.Namespace),
			Score:       0,
			Template:    repo,
		}, nil
	}
	return nil, ErrNoMatch{value: value}
}
