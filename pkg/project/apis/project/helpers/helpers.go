package helpers

import (
	"fmt"

	oapi "github.com/openshift/origin/pkg/api"
	projectapi "github.com/openshift/origin/pkg/project/apis/project"
)

const displayNameOldAnnotation = "displayName"

// DisplayNameAndNameForProject returns a formatted string containing the name
// of the project and includes the display name if it differs.
func DisplayNameAndNameForProject(project *projectapi.Project) string {
	displayName := project.Annotations[oapi.OpenShiftDisplayName]
	if len(displayName) == 0 {
		displayName = project.Annotations[displayNameOldAnnotation]
	}
	if len(displayName) > 0 && displayName != project.Name {
		return fmt.Sprintf("%s (%s)", displayName, project.Name)
	}
	return project.Name
}
