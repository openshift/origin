package validation

import (
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"
	"github.com/openshift/origin/pkg/project/api"
)

// ValidateProject tests required fields for a Project.
func ValidateProject(project *api.Project) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	if len(project.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	} else if !util.IsDNS1123Subdomain(project.Name) {
		result = append(result, fielderrors.NewFieldInvalid("name", project.Name, "does not conform to lower-cased dns1123"))
	}
	if len(project.Namespace) > 0 {
		result = append(result, fielderrors.NewFieldInvalid("namespace", project.Namespace, "must be the empty-string"))
	}
	if !validateNoNewLineOrTab(project.Annotations["displayName"]) {
		result = append(result, fielderrors.NewFieldInvalid("displayName", project.Annotations["displayName"], "may not contain a new line or tab"))
	}
	result = append(result, validateNodeSelector(project)...)
	return result
}

// validateNoNewLineOrTab ensures a string has no new-line or tab
func validateNoNewLineOrTab(s string) bool {
	return !(strings.Contains(s, "\n") || strings.Contains(s, "\t"))
}

// ValidateProjectUpdate tests to make sure a project update can be applied.  Modifies newProject with immutable fields.
func ValidateProjectUpdate(newProject *api.Project, oldProject *api.Project) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, kvalidation.ValidateObjectMetaUpdate(&oldProject.ObjectMeta, &newProject.ObjectMeta).Prefix("metadata")...)
	allErrs = append(allErrs, validateNodeSelector(newProject)...)
	newProject.Spec.Finalizers = oldProject.Spec.Finalizers
	newProject.Status = oldProject.Status
	return allErrs
}

func ValidateProjectRequest(request *api.ProjectRequest) fielderrors.ValidationErrorList {
	project := &api.Project{}
	project.ObjectMeta = request.ObjectMeta

	return ValidateProject(project)
}

func validateNodeSelector(p *api.Project) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(p.Annotations) > 0 {
		if selector, ok := p.Annotations["nodeSelector"]; ok {
			if _, err := labels.Parse(selector); err != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("nodeSelector", p.Annotations["nodeSelector"], "must be a valid label selector"))
			}
		}
	}
	return allErrs
}
