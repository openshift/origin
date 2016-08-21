package app

import (
	"encoding/json"
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
	"github.com/openshift/origin/pkg/template"
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
	for _, term := range terms {
		ref, err := template.ParseTemplateReference(term)
		if err != nil {
			glog.V(2).Infof("template references must be of the form [<namespace>/]<name>, term %q did not qualify", term)
			continue
		}
		if term == "__template_fail" {
			errs = append(errs, fmt.Errorf("unable to find the specified template: %s", term))
			continue
		}

		namespaces := r.Namespaces
		if ref.HasNamespace() {
			namespaces = []string{ref.Namespace}
		}

		checkedNamespaces := sets.NewString()
		for _, namespace := range namespaces {
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
				glog.V(4).Infof("checking namespace %s for template %s", namespace, ref.Name)
				if score, scored := templateScorer(*template, ref.Name); scored {
					if score == 0.0 {
						exact = true
					}
					glog.V(4).Infof("Adding template %q in project %q with score %f", template.Name, template.Namespace, score)
					fullName := fmt.Sprintf("%s/%s", template.Namespace, template.Name)
					matches = append(matches, &ComponentMatch{
						Value:       term,
						Argument:    fmt.Sprintf("--template=%q", fullName),
						Name:        fullName,
						Description: fmt.Sprintf("Template %q in project %q", template.Name, template.Namespace),
						Score:       score,
						Template:    template,
					})
				}
			}

			// If we found one or more exact matches in this namespace, do not continue looking at
			// other namespaces
			if exact && precise {
				break
			}
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
		obj, err := resource.NewBuilder(r.Mapper, r.Typer, r.ClientMapper, kapi.Codecs.UniversalDecoder()).
			NamespaceParam(r.Namespace).RequireNamespace().
			FilenameParam(false, false, term).
			Do().
			IntoSingular(&isSingular).
			Object()

		if err != nil {
			switch {
			case strings.Contains(err.Error(), "does not exist") && strings.Contains(err.Error(), "the path"):
				continue
			default:
				if syntaxErr, ok := err.(*json.SyntaxError); ok {
					err = fmt.Errorf("at offset %d: %v", syntaxErr.Offset, err)
				}
				errs = append(errs, fmt.Errorf("unable to load template file %q: %v", term, err))
				continue
			}
		}

		if list, isList := obj.(*kapi.List); isList && !isSingular {
			if len(list.Items) == 1 {
				obj = list.Items[0]
				isSingular = true
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
