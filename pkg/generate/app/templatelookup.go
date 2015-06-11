package app

import (
	"fmt"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl/resource"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/golang/glog"

	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// TemplateResolver resolves stored template arguments into template objects
type TemplateResolver struct {
	Client                    client.TemplatesNamespacer
	TemplateConfigsNamespacer client.TemplateConfigsNamespacer
	Namespaces                []string
}

// Resolve searches for a template and returns a match with the object representation
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

// IsPossibleTemplateFile returns true if the argument can be a template file
func IsPossibleTemplateFile(value string) bool {
	return isFile(value)
}

// TemplateFileResolver resolves template files into remplate objects
type TemplateFileResolver struct {
	Mapper       meta.RESTMapper
	Typer        runtime.ObjectTyper
	ClientMapper resource.ClientMapper
	Namespace    string
}

// Resolve attemps to read the template file and transform it into a template object
func (r *TemplateFileResolver) Resolve(value string) (*ComponentMatch, error) {
	var isSingular bool
	obj, err := resource.NewBuilder(r.Mapper, r.Typer, r.ClientMapper).
		NamespaceParam(r.Namespace).RequireNamespace().
		FilenameParam(value).
		Do().
		IntoSingular(&isSingular).
		Object()

	if err != nil {
		return nil, err
	}

	if !isSingular {
		return nil, fmt.Errorf("there is more than one object in %q", value)
	}

	template, ok := obj.(*templateapi.Template)
	if !ok {
		return nil, fmt.Errorf("object in %q is not a template", value)
	}

	return &ComponentMatch{
		Value:       value,
		Argument:    fmt.Sprintf("--file=%q", value),
		Name:        value,
		Description: fmt.Sprintf("Template file %s", value),
		Score:       0,
		Template:    template,
	}, nil
}
