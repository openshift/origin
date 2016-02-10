package validation

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api/validation"
	kval "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/intstr"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) field.ErrorList {
	//ensure meta is set properly
	result := kval.ValidateObjectMeta(&route.ObjectMeta, true, oapi.GetNameValidationFunc(kval.ValidatePodName), field.NewPath("metadata"))

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Spec.Host) > 0 {
		if !kvalidation.IsDNS1123Subdomain(route.Spec.Host) {
			result = append(result, field.Invalid(field.NewPath("host"), route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
		}
	}

	if len(route.Spec.Path) > 0 && !strings.HasPrefix(route.Spec.Path, "/") {
		result = append(result, field.Invalid(field.NewPath("path"), route.Spec.Path, "path must begin with /"))
	}

	if len(route.Spec.Path) > 0 && route.Spec.TLS != nil &&
		route.Spec.TLS.Termination == routeapi.TLSTerminationPassthrough {
		result = append(result, field.Invalid(field.NewPath("path"), route.Spec.Path, "passthrough termination does not support paths"))
	}

	if len(route.Spec.To.Name) == 0 {
		result = append(result, field.Required(field.NewPath("serviceName"), ""))
	}

	if route.Spec.Port != nil {
		switch target := route.Spec.Port.TargetPort; {
		case target.Type == intstr.Int && target.IntVal == 0,
			target.Type == intstr.String && len(target.StrVal) == 0:
			result = append(result, field.Required(field.NewPath("targetPort"), ""))
		}
	}

	if errs := validateTLS(route, field.NewPath("tls")); len(errs) != 0 {
		result = append(result, errs...)
	}

	return result
}

func ValidateRouteUpdate(route *routeapi.Route, older *routeapi.Route) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&route.ObjectMeta, &older.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, ValidateRoute(route)...)
	return allErrs
}

// ValidateRouteStatusUpdate validates status updates for routes.
//
// Note that this function shouldn't call ValidateRouteUpdate, otherwise
// we are risking to break existing routes.
func ValidateRouteStatusUpdate(route *routeapi.Route, older *routeapi.Route) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&route.ObjectMeta, &older.ObjectMeta, field.NewPath("metadata"))

	// TODO: validate route status
	return allErrs
}

// validateTLS tests fields for different types of TLS combinations are set.  Called
// by ValidateRoute.
func validateTLS(route *routeapi.Route, fldPath *field.Path) field.ErrorList {
	result := field.ErrorList{}
	tls := route.Spec.TLS

	// no tls config present, no need for validation
	if tls == nil {
		return nil
	}

	switch tls.Termination {
	// reencrypt must specify destination ca cert
	// cert, key, cacert may not be specified because the route may be a wildcard
	case routeapi.TLSTerminationReencrypt:
		if len(tls.DestinationCACertificate) == 0 {
			result = append(result, field.Required(fldPath.Child("destinationCACertificate"), ""))
		}
	//passthrough term should not specify any cert
	case routeapi.TLSTerminationPassthrough:
		if len(tls.Certificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("certificate"), tls.Certificate, "passthrough termination does not support certificates"))
		}

		if len(tls.Key) > 0 {
			result = append(result, field.Invalid(fldPath.Child("key"), tls.Key, "passthrough termination does not support certificates"))
		}

		if len(tls.CACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("caCertificate"), tls.CACertificate, "passthrough termination does not support certificates"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("destinationCACertificate"), tls.DestinationCACertificate, "passthrough termination does not support certificates"))
		}
	// edge cert should only specify cert, key, and cacert but those certs
	// may not be specified if the route is a wildcard route
	case routeapi.TLSTerminationEdge:
		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("destinationCACertificate"), tls.DestinationCACertificate, "edge termination does not support destination certificates"))
		}
	default:
		validValues := []string{string(routeapi.TLSTerminationEdge), string(routeapi.TLSTerminationPassthrough), string(routeapi.TLSTerminationReencrypt)}
		result = append(result, field.NotSupported(fldPath.Child("termination"), tls.Termination, validValues))
	}

	if err := validateInsecureEdgeTerminationPolicy(tls); err != nil {
		result = append(result, err)
	}

	result = append(result, validateNoDoubleEscapes(tls)...)
	return result
}

// validateNoDoubleEscapes ensures double escaped newlines are not in the certificates.  Double
// escaped newlines may be a remnant of old code which used to replace them for the user unnecessarily.
// TODO this is a temporary validation to reject any of our examples with double slashes.  Remove this quickly.
func validateNoDoubleEscapes(tls *routeapi.TLSConfig) field.ErrorList {
	allErrs := field.ErrorList{}
	if strings.Contains(tls.CACertificate, "\\n") {
		allErrs = append(allErrs, field.Invalid(field.NewPath("caCertificate"), tls.CACertificate, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.Certificate, "\\n") {
		allErrs = append(allErrs, field.Invalid(field.NewPath("certificate"), tls.Certificate, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.Key, "\\n") {
		allErrs = append(allErrs, field.Invalid(field.NewPath("key"), tls.Key, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.DestinationCACertificate, "\\n") {
		allErrs = append(allErrs, field.Invalid(field.NewPath("destinationCACertificate"), tls.DestinationCACertificate, `double escaped new lines (\\n) are invalid`))
	}
	return allErrs
}

// validateInsecureEdgeTerminationPolicy tests fields for different types of
// insecure options. Called by validateTLS.
func validateInsecureEdgeTerminationPolicy(tls *routeapi.TLSConfig) *field.Error {
	// Check insecure option value if specified (empty is ok).
	if len(tls.InsecureEdgeTerminationPolicy) == 0 {
		return nil
	}

	// Ensure insecure is set only for edge terminated routes.
	if routeapi.TLSTerminationEdge != tls.Termination {
		// tls.InsecureEdgeTerminationPolicy option is not supported for a non edge-terminated routes.
		return field.Invalid(field.NewPath("insecureEdgeTerminationPolicy"), tls.InsecureEdgeTerminationPolicy, "InsecureEdgeTerminationPolicy is only allowed for edge-terminated routes")
	}

	// It is an edge-terminated route, check insecure option value is
	// one of None(for disable), Allow or Redirect.
	allowedValues := map[routeapi.InsecureEdgeTerminationPolicyType]struct{}{
		routeapi.InsecureEdgeTerminationPolicyNone:     {},
		routeapi.InsecureEdgeTerminationPolicyAllow:    {},
		routeapi.InsecureEdgeTerminationPolicyRedirect: {},
	}

	if _, ok := allowedValues[tls.InsecureEdgeTerminationPolicy]; !ok {
		msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, %s, or empty", routeapi.InsecureEdgeTerminationPolicyNone, routeapi.InsecureEdgeTerminationPolicyAllow, routeapi.InsecureEdgeTerminationPolicyRedirect)
		return field.Invalid(field.NewPath("InsecureEdgeTerminationPolicy"), tls.InsecureEdgeTerminationPolicy, msg)
	}

	return nil
}
