package validation

import (
	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) errs.ValidationErrorList {
	result := errs.ValidationErrorList{}

	if len(route.Host) == 0 {
		result = append(result, errs.NewFieldRequired("host", ""))
	}
	if len(route.ServiceName) == 0 {
		result = append(result, errs.NewFieldRequired("serviceName", ""))
	}
	return result
}
