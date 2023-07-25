package monitorapi

import (
	"strings"
)

// GetNodeRoles extract the node roles from the event message.
func GetNodeRoles(event Interval) string {
	var roles string
	if i := strings.Index(event.Message, "roles/"); i != -1 {
		roles = event.Message[i+len("roles/"):]
		if j := strings.Index(roles, " "); j != -1 {
			roles = roles[:j]
		}
	}

	return roles
}
