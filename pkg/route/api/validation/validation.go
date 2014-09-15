package validation

import (
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) errs.ErrorList {
	result := errs.ErrorList{}

	if len(route.Host) == 0 {
		result = append(result, errs.NewFieldRequired("host", ""))
	}
	if len(route.ServiceName) == 0 {
		result = append(result, errs.NewFieldRequired("serviceName", ""))
	}
	return result
}
