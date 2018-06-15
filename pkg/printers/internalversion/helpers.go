package internalversion

import (
	"time"

	units "github.com/docker/go-units"
	authorizationapi "github.com/openshift/origin/pkg/authorization/apis/authorization"
)

func formatRelativeTime(t time.Time) string {
	return units.HumanDuration(timeNowFn().Sub(t))
}

var timeNowFn = func() time.Time {
	return time.Now()
}

// roleBindingRestrictionType returns a string that indicates the type of the
// given RoleBindingRestriction.
func roleBindingRestrictionType(rbr *authorizationapi.RoleBindingRestriction) string {
	switch {
	case rbr.Spec.UserRestriction != nil:
		return "User"
	case rbr.Spec.GroupRestriction != nil:
		return "Group"
	case rbr.Spec.ServiceAccountRestriction != nil:
		return "ServiceAccount"
	}
	return ""
}
