package validation

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/openshift/origin/pkg/project/api"
	"strings"
)

// ValidateProject tests required fields for a Project.
func ValidateProject(project *api.Project) errors.ErrorList {
	result := errors.ErrorList{}
	if len(project.ID) == 0 {
		result = append(result, errors.NewFieldRequired("ID", project.ID))
	} else if !util.IsDNS952Label(project.ID) {
		result = append(result, errors.NewFieldInvalid("ID", project.ID))
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
