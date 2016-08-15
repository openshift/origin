package validation

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
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

	specPath := field.NewPath("spec")

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Spec.Host) > 0 {
		if len(kvalidation.IsDNS1123Subdomain(route.Spec.Host)) != 0 {
			result = append(result, field.Invalid(specPath.Child("host"), route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
		}
	}

	if len(route.Spec.Path) > 0 && !strings.HasPrefix(route.Spec.Path, "/") {
		result = append(result, field.Invalid(specPath.Child("path"), route.Spec.Path, "path must begin with /"))
	}

	if len(route.Spec.Path) > 0 && route.Spec.TLS != nil &&
		route.Spec.TLS.Termination == routeapi.TLSTerminationPassthrough {
		result = append(result, field.Invalid(specPath.Child("path"), route.Spec.Path, "passthrough termination does not support paths"))
	}

	if len(route.Spec.To.Name) == 0 {
		result = append(result, field.Required(specPath.Child("to", "name"), ""))
	}
	if route.Spec.To.Kind != "Service" {
		result = append(result, field.Invalid(specPath.Child("to", "kind"), route.Spec.To.Kind, "must reference a Service"))
	}
	if route.Spec.To.Weight != nil && (*route.Spec.To.Weight < 0 || *route.Spec.To.Weight > 256) {
		result = append(result, field.Invalid(specPath.Child("to", "weight"), route.Spec.To.Weight, "weight must be an integer between 0 and 256"))
	}

	if len(route.Spec.AlternateBackends) > 3 {
		result = append(result, field.Required(specPath.Child("alternateBackends"), "cannot specify more than 3 additional backends"))
	}
	for _, svc := range route.Spec.AlternateBackends {
		if len(svc.Name) == 0 {
			result = append(result, field.Required(specPath.Child("alternateBackends", "name"), ""))
		}
		if svc.Kind != "Service" {
			result = append(result, field.Invalid(specPath.Child("alternateBackends", "kind"), svc.Kind, "must reference a Service"))
		}
		if svc.Weight != nil && (*svc.Weight < 0 || *svc.Weight > 256) {
			result = append(result, field.Invalid(specPath.Child("alternateBackends", "weight"), svc.Weight, "weight must be an integer between 0 and 256"))
		}
	}

	if route.Spec.Port != nil {
		switch target := route.Spec.Port.TargetPort; {
		case target.Type == intstr.Int && target.IntVal == 0,
			target.Type == intstr.String && len(target.StrVal) == 0:
			result = append(result, field.Required(specPath.Child("port", "targetPort"), ""))
		}
	}

	if errs := validateTLS(route, specPath.Child("tls")); len(errs) != 0 {
		result = append(result, errs...)
	}

	return result
}

func ValidateRouteUpdate(route *routeapi.Route, older *routeapi.Route) field.ErrorList {
	allErrs := validation.ValidateObjectMetaUpdate(&route.ObjectMeta, &older.ObjectMeta, field.NewPath("metadata"))
	allErrs = append(allErrs, validation.ValidateImmutableField(route.Spec.Host, older.Spec.Host, field.NewPath("spec", "host"))...)
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

// ExtendedValidateRoute performs an extended validation on the route
// including checking that the TLS config is valid.
func ExtendedValidateRoute(route *routeapi.Route) field.ErrorList {
	tlsConfig := route.Spec.TLS
	result := field.ErrorList{}

	if tlsConfig == nil {
		return result
	}

	tlsFieldPath := field.NewPath("spec").Child("tls")
	if errs := validateTLS(route, tlsFieldPath); len(errs) != 0 {
		result = append(result, errs...)
	}

	// TODO: Check if we can be stricter with validating the certificate
	//       is for the route hostname. Don't want existing routes to
	//       break, so disable the hostname validation for now.
	// hostname := route.Spec.Host
	hostname := ""
	var certPool *x509.CertPool

	if len(tlsConfig.CACertificate) > 0 {
		certPool = x509.NewCertPool()
		if ok := certPool.AppendCertsFromPEM([]byte(tlsConfig.CACertificate)); !ok {
			result = append(result, field.Invalid(tlsFieldPath.Child("caCertificate"), tlsConfig.CACertificate, "failed to parse CA certificate"))
		}
	}

	verifyOptions := &x509.VerifyOptions{
		DNSName: hostname,
		Roots:   certPool,
	}

	if len(tlsConfig.Certificate) > 0 {
		if _, err := validateCertificatePEM(tlsConfig.Certificate, verifyOptions); err != nil {
			result = append(result, field.Invalid(tlsFieldPath.Child("certificate"), tlsConfig.Certificate, err.Error()))
		}

		certKeyBytes := []byte{}
		certKeyBytes = append(certKeyBytes, []byte(tlsConfig.Certificate)...)
		if len(tlsConfig.Key) > 0 {
			certKeyBytes = append(certKeyBytes, byte('\n'))
			certKeyBytes = append(certKeyBytes, []byte(tlsConfig.Key)...)
		}

		if _, err := tls.X509KeyPair(certKeyBytes, certKeyBytes); err != nil {
			result = append(result, field.Invalid(tlsFieldPath.Child("key"), tlsConfig.Key, err.Error()))
		}
	}

	if len(tlsConfig.DestinationCACertificate) > 0 {
		roots := x509.NewCertPool()
		if ok := roots.AppendCertsFromPEM([]byte(tlsConfig.DestinationCACertificate)); !ok {
			result = append(result, field.Invalid(tlsFieldPath.Child("destinationCACertificate"), tlsConfig.DestinationCACertificate, "failed to parse destination CA certificate"))
		}
	}

	return result
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

	if err := validateInsecureEdgeTerminationPolicy(tls, fldPath.Child("insecureEdgeTerminationPolicy")); err != nil {
		result = append(result, err)
	}

	return result
}

// validateInsecureEdgeTerminationPolicy tests fields for different types of
// insecure options. Called by validateTLS.
func validateInsecureEdgeTerminationPolicy(tls *routeapi.TLSConfig, fldPath *field.Path) *field.Error {
	// Check insecure option value if specified (empty is ok).
	if len(tls.InsecureEdgeTerminationPolicy) == 0 {
		return nil
	}

	// Ensure insecure is set only for edge terminated routes.
	if routeapi.TLSTerminationEdge != tls.Termination {
		// tls.InsecureEdgeTerminationPolicy option is not supported for a non edge-terminated routes.
		return field.Invalid(fldPath, tls.InsecureEdgeTerminationPolicy, "InsecureEdgeTerminationPolicy is only allowed for edge-terminated routes")
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
		return field.Invalid(fldPath, tls.InsecureEdgeTerminationPolicy, msg)
	}

	return nil
}

// validateCertificatePEM checks if a certificate PEM is valid and
// optionally verifies the certificate using the options.
func validateCertificatePEM(certPEM string, options *x509.VerifyOptions) (*x509.Certificate, error) {
	var data *pem.Block
	for remaining := []byte(certPEM); len(remaining) > 0; {
		block, rest := pem.Decode(remaining)
		if block == nil {
			return nil, fmt.Errorf("error decoding certificate data")
		}
		if block.Type == "CERTIFICATE" {
			data = block
			break
		}
		remaining = rest
	}

	if data == nil || len(data.Bytes) < 1 {
		return nil, fmt.Errorf("invalid/empty certificate data")
	}

	cert, err := x509.ParseCertificate(data.Bytes)
	if err != nil {
		return nil, fmt.Errorf("error parsing certificate: %s", err.Error())
	}

	if options != nil {
		_, err = cert.Verify(*options)
		if err != nil {
			return cert, fmt.Errorf("error verifying certificate: %s", err.Error())
		}
	}

	return cert, nil
}
