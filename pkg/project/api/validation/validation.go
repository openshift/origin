package validation

import (
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/project/api"
	"github.com/openshift/origin/pkg/util/labelselector"
)

// ValidateProject tests required fields for a Project.
// This should only be called when creating a project (not on update),
// since its name validation is more restrictive than default namespace name validation
func ValidateProject(project *api.Project) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	if len(project.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("name"))
	} else if ok, msg := kvalidation.ValidateNamespaceName(project.Name, false); !ok {
		result = append(result, fielderrors.NewFieldInvalid("name", project.Name, msg))
	} else if len(project.Name) < 2 {
		// Ensure projects can serve as namespaces for the internal docker registry
		result = append(result, fielderrors.NewFieldInvalid("name", project.Name, "must be at least 2 characters long"))
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
		if selector, ok := p.Annotations[api.ProjectNodeSelectorParam]; ok {
			if _, err := labelselector.Parse(selector); err != nil {
				allErrs = append(allErrs, fielderrors.NewFieldInvalid("nodeSelector", p.Annotations[api.ProjectNodeSelectorParam], "must be a valid label selector"))
			}
		}
	}
	return allErrs
}
