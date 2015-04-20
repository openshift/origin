package validation

import (
	"strings"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
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
	if !validateNoNewLineOrTab(project.DisplayName) {
		result = append(result, fielderrors.NewFieldInvalid("displayName", project.DisplayName, "may not contain a new line or tab"))
	}
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
	newProject.Spec.Finalizers = oldProject.Spec.Finalizers
	newProject.Status = oldProject.Status
	return allErrs
}

func ValidateProjectRequest(request *api.ProjectRequest) fielderrors.ValidationErrorList {
	project := &api.Project{}
	project.ObjectMeta = request.ObjectMeta
	project.DisplayName = request.DisplayName

	return ValidateProject(project)
}
