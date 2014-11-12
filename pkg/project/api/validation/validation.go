package validation

import (
	"strings"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/project/api"
)

// ValidateProject tests required fields for a Project.
func ValidateProject(project *api.Project) errors.ValidationErrorList {
	result := errors.ValidationErrorList{}
	if len(project.Name) == 0 {
		result = append(result, errors.NewFieldRequired("Name", project.Name))
	} else if !util.IsDNS952Label(project.Name) {
		result = append(result, errors.NewFieldInvalid("Name", project.Name))
	}
	if !util.IsDNSSubdomain(project.Namespace) {
		result = append(result, errors.NewFieldInvalid("Namespace", project.Namespace))
	}
	if !validateNoNewLineOrTab(project.DisplayName) {
		result = append(result, errors.NewFieldInvalid("DisplayName", project.DisplayName))
	}
	if !validateNoNewLineOrTab(project.Description) {
		result = append(result, errors.NewFieldInvalid("Description", project.Description))
	}
	return result
}

// validateNoNewLineOrTab ensures a string has no new-line or tab
func validateNoNewLineOrTab(s string) bool {
	return !(strings.Contains(s, "\n") || strings.Contains(s, "\t"))
}
