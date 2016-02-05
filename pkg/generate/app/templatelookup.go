package app

import (
	"fmt"
	"strings"

	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/meta"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/client"
	templateapi "github.com/openshift/origin/pkg/template/api"
)

// TemplateSearcher resolves stored template arguments into template objects
type TemplateSearcher struct {
	Client                    client.TemplatesNamespacer
	TemplateConfigsNamespacer client.TemplateConfigsNamespacer
	Namespaces                []string
	StopOnExactMatch          bool
}

// Search searches for a template and returns matches with the object representation
func (r TemplateSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	matches := ComponentMatches{}
	var errs []error
	checkedNamespaces := sets.NewString()
	for _, namespace := range r.Namespaces {
		if checkedNamespaces.Has(namespace) {
			continue
		}
		checkedNamespaces.Insert(namespace)

		templates, err := r.Client.Templates(namespace).List(kapi.ListOptions{})
		if err != nil {
			if errors.IsNotFound(err) || errors.IsForbidden(err) {
				continue
			}
			errs = append(errs, err)
			continue
		}

		exact := false
		for i := range templates.Items {
			template := &templates.Items[i]
			for _, term := range terms {
				if term == "__template_fail" {
					errs = append(errs, fmt.Errorf("unable to find the specified template: %s", term))
					continue
				}

				glog.V(4).Infof("checking for term %s in namespace %s", term, namespace)
				if score, scored := templateScorer(*template, term); scored {
					if score == 0.0 {
						exact = true
					}
					glog.V(4).Infof("Adding template %q in project %q with score %f", template.Name, template.Namespace, score)
					matches = append(matches, &ComponentMatch{
						Value:       term,
						Argument:    fmt.Sprintf("--template=%q", template.Name),
						Name:        template.Name,
						Description: fmt.Sprintf("Template %q in project %q", template.Name, template.Namespace),
						Score:       score,
						Template:    template,
					})
				}
			}
		}

		// If we found one or more exact matches in this namespace, do not continue looking at
		// other namespaces
		if exact && precise {
			break
		}
	}

	return matches, errs
}

// IsPossibleTemplateFile returns true if the argument can be a template file
func IsPossibleTemplateFile(value string) bool {
	return isFile(value)
}

// TemplateFileSearcher resolves template files into template objects
type TemplateFileSearcher struct {
	Mapper       meta.RESTMapper
	Typer        runtime.ObjectTyper
	ClientMapper resource.ClientMapper
	Namespace    string
}

// Search attemps to read template files and transform it into template objects
func (r *TemplateFileSearcher) Search(precise bool, terms ...string) (ComponentMatches, []error) {
	matches := ComponentMatches{}
	var errs []error
	for _, term := range terms {
		if term == "__templatefile_fail" {
			errs = append(errs, fmt.Errorf("unable to find the specified template file: %s", term))
			continue
		}

		var isSingular bool
		obj, err := resource.NewBuilder(r.Mapper, r.Typer, r.ClientMapper).
			NamespaceParam(r.Namespace).RequireNamespace().
			FilenameParam(false, term).
			Do().
			IntoSingular(&isSingular).
			Object()

		if err != nil {
			switch {
			case strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "the path"):
				continue
			default:
				errs = append(errs, fmt.Errorf("unable to load template file %q: %v", term, err))
				continue
			}
		}

		if !isSingular {
			errs = append(errs, fmt.Errorf("there is more than one object in %q", term))
			continue
		}

		template, ok := obj.(*templateapi.Template)
		if !ok {
			errs = append(errs, fmt.Errorf("object in %q is not a template", term))
			continue
		}

		matches = append(matches, &ComponentMatch{
			Value:       term,
			Argument:    fmt.Sprintf("--file=%q", template.Name),
			Name:        template.Name,
			Description: fmt.Sprintf("Template file %s", term),
			Score:       0,
			Template:    template,
		})
	}

	return matches, errs
}
