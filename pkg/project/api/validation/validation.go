package validation

import (
	"strings"

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
