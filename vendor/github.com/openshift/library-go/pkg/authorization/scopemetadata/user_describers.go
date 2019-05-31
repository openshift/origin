package scopelibrary

import (
	"fmt"
)

// these must agree with the scope authorizer, but it's an API we cannot realistically change
const (
	scopesAllNamespaces = "*"

	userIndicator        = "user:"
	clusterRoleIndicator = "role:"

	userInfo        = userIndicator + "info"
	userAccessCheck = userIndicator + "check-access"

	// UserListScopedProjects gives explicit permission to see the projects that this token can see.
	userListScopedProjects = userIndicator + "list-scoped-projects"

	// UserListAllProjects gives explicit permission to see the projects a user can see.  This is often used to prime secondary ACL systems
	// unrelated to openshift and to display projects for selection in a secondary UI.
	userListAllProjects = userIndicator + "list-projects"

	// UserFull includes all permissions of the user
	userFull = userIndicator + "full"
)

// user:<scope name>
type UserEvaluator struct{}

func (UserEvaluator) Handles(scope string) bool {
	return UserEvaluatorHandles(scope)
}

func (e UserEvaluator) Validate(scope string) error {
	if e.Handles(scope) {
		return nil
	}

	return fmt.Errorf("unrecognized scope: %v", scope)
}

var defaultSupportedScopesMap = map[string]string{
	userInfo:               "Read-only access to your user information (including username, identities, and group membership)",
	userAccessCheck:        `Read-only access to view your privileges (for example, "can I create builds?")`,
	userListScopedProjects: `Read-only access to list your projects viewable with this token and view their metadata (display name, description, etc.)`,
	userListAllProjects:    `Read-only access to list your projects and view their metadata (display name, description, etc.)`,
	userFull:               `Full read/write access with all of your permissions`,
}

func (UserEvaluator) Describe(scope string) (string, string, error) {
	switch scope {
	case userInfo, userAccessCheck, userListScopedProjects, userListAllProjects:
		return defaultSupportedScopesMap[scope], "", nil
	case userFull:
		return defaultSupportedScopesMap[scope], `Includes any access you have to escalating resources like secrets`, nil
	default:
		return "", "", fmt.Errorf("unrecognized scope: %v", scope)
	}
}

func UserEvaluatorHandles(scope string) bool {
	switch scope {
	case userFull, userInfo, userAccessCheck, userListScopedProjects, userListAllProjects:
		return true
	}
	return false
}
