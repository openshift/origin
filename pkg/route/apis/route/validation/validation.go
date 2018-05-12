package validation

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/apimachinery/pkg/util/sets"
	kvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/cert"
	"k8s.io/kubernetes/pkg/apis/core/validation"

	routeapi "github.com/openshift/origin/pkg/route/apis/route"
)

var ValidateRouteName = validation.NameIsDNSSubdomain

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) field.ErrorList {
	//ensure meta is set properly
	result := validation.ValidateObjectMeta(&route.ObjectMeta, true, ValidateRouteName, field.NewPath("metadata"))

	specPath := field.NewPath("spec")

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Spec.Host) > 0 {
		// TODO: Add a better check that the host name matches up to
		//       DNS requirements. Change to use:
		//         ValidateHostName(route)
		//       Need to check the implications of doing it here in
		//       ValidateRoute - probably needs to be done only on
		//       creation time for new routes.
		if len(kvalidation.IsDNS1123Subdomain(route.Spec.Host)) != 0 {
			result = append(result, field.Invalid(specPath.Child("host"), route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
		}
	}

	if err := validateWildcardPolicy(route.Spec.Host, route.Spec.WildcardPolicy, specPath.Child("wildcardPolicy")); err != nil {
		result = append(result, err)
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

	backendPath := specPath.Child("alternateBackends")
	if len(route.Spec.AlternateBackends) > 3 {
		result = append(result, field.Required(backendPath, "cannot specify more than 3 alternate backends"))
	}
	for i, svc := range route.Spec.AlternateBackends {
		if len(svc.Name) == 0 {
			result = append(result, field.Required(backendPath.Index(i).Child("name"), ""))
		}
		if svc.Kind != "Service" {
			result = append(result, field.Invalid(backendPath.Index(i).Child("kind"), svc.Kind, "must reference a Service"))
		}
		if svc.Weight != nil && (*svc.Weight < 0 || *svc.Weight > 256) {
			result = append(result, field.Invalid(backendPath.Index(i).Child("weight"), svc.Weight, "weight must be an integer between 0 and 256"))
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
	allErrs = append(allErrs, validation.ValidateImmutableField(route.Spec.WildcardPolicy, older.Spec.WildcardPolicy, field.NewPath("spec", "wildcardPolicy"))...)
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

type blockVerifierFunc func(block *pem.Block) (*pem.Block, error)

func publicKeyBlockVerifier(block *pem.Block) (*pem.Block, error) {
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	block = &pem.Block{
		Type: "PUBLIC KEY",
	}
	if block.Bytes, err = x509.MarshalPKIXPublicKey(key); err != nil {
		return nil, err
	}
	return block, nil
}

func certificateBlockVerifier(block *pem.Block) (*pem.Block, error) {
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, err
	}
	block = &pem.Block{
		Type:  "CERTIFICATE",
		Bytes: cert.Raw,
	}
	return block, nil
}

func privateKeyBlockVerifier(block *pem.Block) (*pem.Block, error) {
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		key, err = x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			key, err = x509.ParseECPrivateKey(block.Bytes)
			if err != nil {
				return nil, fmt.Errorf("block %s is not valid", block.Type)
			}
		}
	}
	switch t := key.(type) {
	case *rsa.PrivateKey:
		block = &pem.Block{
			Type:  "RSA PRIVATE KEY",
			Bytes: x509.MarshalPKCS1PrivateKey(t),
		}
	case *ecdsa.PrivateKey:
		block = &pem.Block{
			Type: "ECDSA PRIVATE KEY",
		}
		if block.Bytes, err = x509.MarshalECPrivateKey(t); err != nil {
			return nil, err
		}
	default:
		return nil, fmt.Errorf("block private key %T is not valid", key)
	}
	return block, nil
}

func ignoreBlockVerifier(block *pem.Block) (*pem.Block, error) {
	return nil, nil
}

var knownBlockDecoders = map[string]blockVerifierFunc{
	"RSA PRIVATE KEY":   privateKeyBlockVerifier,
	"ECDSA PRIVATE KEY": privateKeyBlockVerifier,
	"PRIVATE KEY":       privateKeyBlockVerifier,
	"PUBLIC KEY":        publicKeyBlockVerifier,
	// Potential "in the wild" PEM encoded blocks that can be normalized
	"RSA PUBLIC KEY":   publicKeyBlockVerifier,
	"DSA PUBLIC KEY":   publicKeyBlockVerifier,
	"ECDSA PUBLIC KEY": publicKeyBlockVerifier,
	"CERTIFICATE":      certificateBlockVerifier,
	// Blocks that should be dropped
	"EC PARAMETERS": ignoreBlockVerifier,
}

// sanitizePEM takes a block of data that should be encoded in PEM and returns only
// the parts of it that parse and serialize as valid recognized certs in valid PEM blocks.
// We perform this transformation to eliminate potentially incorrect / invalid PEM contents
// to prevent OpenSSL or other non Golang tools from receiving unsanitized input.
func sanitizePEM(data []byte) ([]byte, error) {
	var block *pem.Block
	buf := &bytes.Buffer{}
	for len(data) > 0 {
		block, data = pem.Decode(data)
		if block == nil {
			return buf.Bytes(), nil
		}
		fn, ok := knownBlockDecoders[block.Type]
		if !ok {
			return nil, fmt.Errorf("unrecognized PEM block %s", block.Type)
		}
		newBlock, err := fn(block)
		if err != nil {
			return nil, err
		}
		if newBlock == nil {
			continue
		}
		if err := pem.Encode(buf, newBlock); err != nil {
			return nil, err
		}
	}
	return buf.Bytes(), nil
}

// ExtendedValidateRoute performs an extended validation on the route
// including checking that the TLS config is valid. It also sanitizes
// the contents of valid certificates by removing any data that
// is not recognizable PEM blocks on the incoming route.
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
	var verifyOptions *x509.VerifyOptions

	if len(tlsConfig.CACertificate) > 0 {
		certPool := x509.NewCertPool()
		if certs, err := cert.ParseCertsPEM([]byte(tlsConfig.CACertificate)); err != nil {
			errmsg := fmt.Sprintf("failed to parse CA certificate: %v", err)
			result = append(result, field.Invalid(tlsFieldPath.Child("caCertificate"), "redacted ca certificate data", errmsg))
		} else {
			for _, cert := range certs {
				certPool.AddCert(cert)
			}
			if data, err := sanitizePEM([]byte(tlsConfig.CACertificate)); err != nil {
				result = append(result, field.Invalid(tlsFieldPath.Child("caCertificate"), "redacted ca certificate data", err.Error()))
			} else {
				tlsConfig.CACertificate = string(data)
			}
		}

		verifyOptions = &x509.VerifyOptions{
			DNSName:       hostname,
			Intermediates: certPool,
			Roots:         certPool,
		}
	}

	if len(tlsConfig.Certificate) > 0 {
		if _, err := validateCertificatePEM(tlsConfig.Certificate, verifyOptions); err != nil {
			result = append(result, field.Invalid(tlsFieldPath.Child("certificate"), "redacted certificate data", err.Error()))
		} else {
			if data, err := sanitizePEM([]byte(tlsConfig.Certificate)); err != nil {
				result = append(result, field.Invalid(tlsFieldPath.Child("certificate"), "redacted certificate data", err.Error()))
			} else {
				tlsConfig.Certificate = string(data)
			}
		}

		certKeyBytes := []byte{}
		certKeyBytes = append(certKeyBytes, []byte(tlsConfig.Certificate)...)
		if len(tlsConfig.Key) > 0 {
			certKeyBytes = append(certKeyBytes, byte('\n'))
			certKeyBytes = append(certKeyBytes, []byte(tlsConfig.Key)...)
		}

		if _, err := tls.X509KeyPair(certKeyBytes, certKeyBytes); err != nil {
			result = append(result, field.Invalid(tlsFieldPath.Child("key"), "redacted key data", err.Error()))
		}
	}

	if len(tlsConfig.Key) > 0 {
		if data, err := sanitizePEM([]byte(tlsConfig.Key)); err != nil {
			result = append(result, field.Invalid(tlsFieldPath.Child("key"), "redacted key data", err.Error()))
		} else {
			tlsConfig.Key = string(data)
		}
	}

	if len(tlsConfig.DestinationCACertificate) > 0 {
		if _, err := cert.ParseCertsPEM([]byte(tlsConfig.DestinationCACertificate)); err != nil {
			errmsg := fmt.Sprintf("failed to parse destination CA certificate: %v", err)
			result = append(result, field.Invalid(tlsFieldPath.Child("destinationCACertificate"), "redacted destination ca certificate data", errmsg))
		} else {
			if data, err := sanitizePEM([]byte(tlsConfig.DestinationCACertificate)); err != nil {
				result = append(result, field.Invalid(tlsFieldPath.Child("destinationCACertificate"), "redacted destination ca certificate data", err.Error()))
			} else {
				tlsConfig.DestinationCACertificate = string(data)
			}
		}
	}

	return result
}

// ValidateHostName checks that a route's host name satisfies DNS requirements.
func ValidateHostName(route *routeapi.Route) field.ErrorList {
	result := field.ErrorList{}
	if len(route.Spec.Host) < 1 {
		return result
	}

	specPath := field.NewPath("spec")
	hostPath := specPath.Child("host")

	if len(kvalidation.IsDNS1123Subdomain(route.Spec.Host)) != 0 {
		result = append(result, field.Invalid(hostPath, route.Spec.Host, "host must conform to DNS 952 subdomain conventions"))
	}

	segments := strings.Split(route.Spec.Host, ".")
	for _, s := range segments {
		errs := kvalidation.IsDNS1123Label(s)
		for _, e := range errs {
			result = append(result, field.Invalid(hostPath, route.Spec.Host, e))
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
	// reencrypt may specify destination ca cert
	// cert, key, cacert may not be specified because the route may be a wildcard
	case routeapi.TLSTerminationReencrypt:
	//passthrough term should not specify any cert
	case routeapi.TLSTerminationPassthrough:
		if len(tls.Certificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("certificate"), "redacted certificate data", "passthrough termination does not support certificates"))
		}

		if len(tls.Key) > 0 {
			result = append(result, field.Invalid(fldPath.Child("key"), "redacted key data", "passthrough termination does not support certificates"))
		}

		if len(tls.CACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("caCertificate"), "redacted ca certificate data", "passthrough termination does not support certificates"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("destinationCACertificate"), "redacted destination ca certificate data", "passthrough termination does not support certificates"))
		}
	// edge cert should only specify cert, key, and cacert but those certs
	// may not be specified if the route is a wildcard route
	case routeapi.TLSTerminationEdge:
		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("destinationCACertificate"), "redacted destination ca certificate data", "edge termination does not support destination certificates"))
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

	// It is an edge-terminated or reencrypt route, check insecure option value is
	// one of None(for disable), Allow or Redirect.
	allowedValues := map[routeapi.InsecureEdgeTerminationPolicyType]struct{}{
		routeapi.InsecureEdgeTerminationPolicyNone:     {},
		routeapi.InsecureEdgeTerminationPolicyAllow:    {},
		routeapi.InsecureEdgeTerminationPolicyRedirect: {},
	}

	switch tls.Termination {
	case routeapi.TLSTerminationReencrypt:
		fallthrough
	case routeapi.TLSTerminationEdge:
		if _, ok := allowedValues[tls.InsecureEdgeTerminationPolicy]; !ok {
			msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, %s, or empty", routeapi.InsecureEdgeTerminationPolicyNone, routeapi.InsecureEdgeTerminationPolicyAllow, routeapi.InsecureEdgeTerminationPolicyRedirect)
			return field.Invalid(fldPath, tls.InsecureEdgeTerminationPolicy, msg)
		}
	case routeapi.TLSTerminationPassthrough:
		if routeapi.InsecureEdgeTerminationPolicyNone != tls.InsecureEdgeTerminationPolicy && routeapi.InsecureEdgeTerminationPolicyRedirect != tls.InsecureEdgeTerminationPolicy {
			msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, or empty", routeapi.InsecureEdgeTerminationPolicyNone, routeapi.InsecureEdgeTerminationPolicyRedirect)
			return field.Invalid(fldPath, tls.InsecureEdgeTerminationPolicy, msg)
		}
	}

	return nil
}

// validateCertificatePEM checks if a certificate PEM is valid and
// optionally verifies the certificate using the options.
func validateCertificatePEM(certPEM string, options *x509.VerifyOptions) ([]*x509.Certificate, error) {
	certs, err := cert.ParseCertsPEM([]byte(certPEM))
	if err != nil {
		return nil, err
	}

	if len(certs) < 1 {
		return nil, fmt.Errorf("invalid/empty certificate data")
	}

	if options != nil {
		// Ensure we don't report errors for expired certs or if
		// the validity is in the future.
		// Not that this can be for the actual certificate or any
		// intermediates in the CA chain. This allows the router to
		// still serve an expired/valid-in-the-future certificate
		// and lets the client to control if it can tolerate that
		// (just like for self-signed certs).
		_, err = certs[0].Verify(*options)
		if err != nil {
			if invalidErr, ok := err.(x509.CertificateInvalidError); !ok || invalidErr.Reason != x509.Expired {
				return certs, fmt.Errorf("error verifying certificate: %s", err.Error())
			}
		}
	}

	return certs, nil
}

var (
	allowedWildcardPolicies    = []string{string(routeapi.WildcardPolicyNone), string(routeapi.WildcardPolicySubdomain)}
	allowedWildcardPoliciesSet = sets.NewString(allowedWildcardPolicies...)
)

// validateWildcardPolicy tests that the wildcard policy is either empty or one of the supported types.
func validateWildcardPolicy(host string, policy routeapi.WildcardPolicyType, fldPath *field.Path) *field.Error {
	if len(policy) == 0 {
		return nil
	}

	// Check if policy is one of None or Subdomain.
	if !allowedWildcardPoliciesSet.Has(string(policy)) {
		return field.NotSupported(fldPath, policy, allowedWildcardPolicies)
	}

	if policy == routeapi.WildcardPolicySubdomain && len(host) == 0 {
		return field.Invalid(fldPath, policy, "host name not specified for wildcard policy")
	}

	return nil
}
