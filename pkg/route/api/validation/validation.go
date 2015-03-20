package validation

import (
	"strings"

	errs "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	kval "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) errs.ValidationErrorList {
	result := errs.ValidationErrorList{}

	//ensure meta is set properly
	result = append(result, kval.ValidateObjectMeta(&route.ObjectMeta, true, kval.ValidatePodName)...)

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Host) > 0 {
		if !util.IsDNS1123Subdomain(route.Host) {
			result = append(result, errs.NewFieldInvalid("host", route.Host, "Host must conform to DNS 952 subdomain conventions"))
		}
	}

	if len(route.Path) > 0 && !strings.HasPrefix(route.Path, "/") {
		result = append(result, errs.NewFieldInvalid("path", route.Path, "Path must begin with /"))
	}

	if len(route.ServiceName) == 0 {
		result = append(result, errs.NewFieldRequired("serviceName"))
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
			result = append(result, errs.NewFieldRequired("certificate"))
		}

		if len(tls.Key) == 0 {
			result = append(result, errs.NewFieldRequired("key"))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("caCertificate"))
		}

		if len(tls.DestinationCACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("destinationCACertificate"))
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
			result = append(result, errs.NewFieldRequired("certificate"))
		}

		if len(tls.Key) == 0 {
			result = append(result, errs.NewFieldRequired("key"))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, errs.NewFieldRequired("caCertificate"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, errs.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "edge termination does not support destination certificates"))
		}
	}

	return result
}
