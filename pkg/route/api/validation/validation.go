package validation

import (
	"fmt"
	"strings"

	"k8s.io/kubernetes/pkg/api/validation"
	kval "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	kvalidation "k8s.io/kubernetes/pkg/util/validation"

	oapi "github.com/openshift/origin/pkg/api"
	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	//ensure meta is set properly
	result = append(result, kval.ValidateObjectMeta(&route.ObjectMeta, true, oapi.GetNameValidationFunc(kval.ValidatePodName)).Prefix("metadata")...)

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Spec.Host) > 0 {
		if !kvalidation.IsDNS1123Subdomain(route.Spec.Host) {
			result = append(result, fielderrors.NewFieldInvalid("host", route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
		}
	}

	if len(route.Spec.Path) > 0 && !strings.HasPrefix(route.Spec.Path, "/") {
		result = append(result, fielderrors.NewFieldInvalid("path", route.Spec.Path, "path must begin with /"))
	}

	if len(route.Spec.To.Name) == 0 {
		result = append(result, fielderrors.NewFieldRequired("serviceName"))
	}

	if route.Spec.Port != nil {
		switch target := route.Spec.Port.TargetPort; {
		case target.Kind == util.IntstrInt && target.IntVal == 0,
			target.Kind == util.IntstrString && len(target.StrVal) == 0:
			result = append(result, fielderrors.NewFieldRequired("targetPort"))
		}
	}

	if errs := validateTLS(route); len(errs) != 0 {
		result = append(result, errs.Prefix("tls")...)
	}

	return result
}

func ValidateRouteUpdate(route *routeapi.Route, older *routeapi.Route) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&route.ObjectMeta, &older.ObjectMeta).Prefix("metadata")...)

	allErrs = append(allErrs, ValidateRoute(route)...)
	return allErrs
}

func ValidateRouteStatusUpdate(route *routeapi.Route, older *routeapi.Route) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	allErrs = append(allErrs, validation.ValidateObjectMetaUpdate(&route.ObjectMeta, &older.ObjectMeta).Prefix("metadata")...)

	// TODO: validate route status
	return allErrs
}

// ValidateTLS tests fields for different types of TLS combinations are set.  Called
// by ValidateRoute.
func validateTLS(route *routeapi.Route) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	tls := route.Spec.TLS

	//no termination, ignore other settings
	if tls == nil || tls.Termination == "" {
		return nil
	}

	switch tls.Termination {
	// reencrypt must specify destination ca cert
	// cert, key, cacert may not be specified because the route may be a wildcard
	case routeapi.TLSTerminationReencrypt:
		if len(tls.DestinationCACertificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("destinationCACertificate"))
		}
	//passthrough term should not specify any cert
	case routeapi.TLSTerminationPassthrough:
		if len(tls.Certificate) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("certificate", tls.Certificate, "passthrough termination does not support certificates"))
		}

		if len(tls.Key) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("key", tls.Key, "passthrough termination does not support certificates"))
		}

		if len(tls.CACertificate) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("caCertificate", tls.CACertificate, "passthrough termination does not support certificates"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "passthrough termination does not support certificates"))
		}
	// edge cert should only specify cert, key, and cacert but those certs
	// may not be specified if the route is a wildcard route
	case routeapi.TLSTerminationEdge:
		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "edge termination does not support destination certificates"))
		}
	default:
		msg := fmt.Sprintf("invalid value for termination, acceptable values are %s, %s, %s, or empty (no tls specified)", routeapi.TLSTerminationEdge, routeapi.TLSTerminationPassthrough, routeapi.TLSTerminationReencrypt)
		result = append(result, fielderrors.NewFieldInvalid("termination", tls.Termination, msg))
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
func validateNoDoubleEscapes(tls *routeapi.TLSConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if strings.Contains(tls.CACertificate, "\\n") {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("caCertificate", tls.CACertificate, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.Certificate, "\\n") {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("certificate", tls.Certificate, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.Key, "\\n") {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("key", tls.Key, `double escaped new lines (\\n) are invalid`))
	}
	if strings.Contains(tls.DestinationCACertificate, "\\n") {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, `double escaped new lines (\\n) are invalid`))
	}
	return allErrs
}

// validateInsecureEdgeTerminationPolicy tests fields for different types of
// insecure options. Called by validateTLS.
func validateInsecureEdgeTerminationPolicy(tls *routeapi.TLSConfig) *fielderrors.ValidationError {
	// Check insecure option value if specified (empty is ok).
	if len(tls.InsecureEdgeTerminationPolicy) == 0 {
		return nil
	}

	// Ensure insecure is set only for edge terminated routes.
	if routeapi.TLSTerminationEdge != tls.Termination {
		// tls.InsecureEdgeTerminationPolicy option is not supported for a non edge-terminated routes.
		return fielderrors.NewFieldInvalid("InsecureEdgeTerminationPolicy", tls.InsecureEdgeTerminationPolicy, "InsecureEdgeTerminationPolicy is only allowed for edge-terminated routes")
	}

	// It is an edge-terminated route, check insecure option value is
	// one of none(or disable), allow or redirect.
	allowedValues := map[routeapi.InsecureEdgeTerminationPolicyType]bool{
		routeapi.InsecureEdgeTerminationPolicyNone:     true,
		routeapi.InsecureEdgeTerminationPolicyAllow:    true,
		routeapi.InsecureEdgeTerminationPolicyRedirect: true,
	}

	if _, ok := allowedValues[tls.InsecureEdgeTerminationPolicy]; !ok {
		msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, %s, or empty", routeapi.InsecureEdgeTerminationPolicyNone, routeapi.InsecureEdgeTerminationPolicyAllow, routeapi.InsecureEdgeTerminationPolicyRedirect)
		return fielderrors.NewFieldInvalid("InsecureEdgeTerminationPolicy", tls.InsecureEdgeTerminationPolicy, msg)
	}

	return nil
}
