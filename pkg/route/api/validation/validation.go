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

	if errs := validateTLS(route.TLS); len(errs) != 0 {
		result = append(result, errs.Prefix("tls")...)
	}

	return result
}

// ValidateTLS tests fields for different types of TLS combinations are set.  Called
// by ValidateRoute.
func validateTLS(tls *routeapi.TLSConfig) errs.ValidationErrorList {
	result := errs.ValidationErrorList{}

	//no termination, ignore other settings
	if tls == nil || tls.Termination == "" {
		return nil
	}

	//reencrypt must specify cert, key, cacert, and destination ca cert
	if tls.Termination == routeapi.TLSTerminationReencrypt {
		if len(tls.Certificate) == 0 {
			result = append(result, errs.NewFieldRequired("certificate", tls.Certificate))
		}

		if len(tls.Key) == 0 {
			result = append(result, errs.NewFieldRequired("key", tls.Key))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("caCertificate", tls.CACertificate))
		}

		if len(tls.DestinationCACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("destinationCACertificate", tls.DestinationCACertificate))
		}
	}

	//passthrough term should not specify any cert
	if tls.Termination == routeapi.TLSTerminationPassthrough {
		if len(tls.Certificate) > 0 {
			result = append(result, errs.NewFieldInvalid("certificate", tls.Certificate, "passthrough termination does not support certificates"))
		}

		if len(tls.Key) > 0 {
			result = append(result, errs.NewFieldInvalid("key", tls.Key, "passthrough termination does not support certificates"))
		}

		if len(tls.CACertificate) > 0 {
			result = append(result, errs.NewFieldInvalid("caCertificate", tls.CACertificate, "passthrough termination does not support certificates"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, errs.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "passthrough termination does not support certificates"))
		}
	}

	//edge cert should specify cert, key, and cacert
	if tls.Termination == routeapi.TLSTerminationEdge {
		if len(tls.Certificate) == 0 {
			result = append(result, errs.NewFieldRequired("certificate", tls.Certificate))
		}

		if len(tls.Key) == 0 {
			result = append(result, errs.NewFieldRequired("key", tls.Key))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("caCertificate", tls.CACertificate))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, errs.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "edge termination does not support destination certificates"))
		}
	}

	return result
}
