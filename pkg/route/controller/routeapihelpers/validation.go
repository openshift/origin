package routeapihelpers

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"

	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/client-go/util/cert"

	routev1 "github.com/openshift/api/route/v1"
)

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
			Type: "EC PRIVATE KEY",
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
	"EC PRIVATE KEY":    privateKeyBlockVerifier,
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
func ExtendedValidateRoute(route *routev1.Route) field.ErrorList {
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

// validateTLS tests fields for different types of TLS combinations are set.  Called
// by ValidateRoute.
func validateTLS(route *routev1.Route, fldPath *field.Path) field.ErrorList {
	result := field.ErrorList{}
	tls := route.Spec.TLS

	// no tls config present, no need for validation
	if tls == nil {
		return nil
	}

	switch tls.Termination {
	// reencrypt may specify destination ca cert
	// cert, key, cacert may not be specified because the route may be a wildcard
	case routev1.TLSTerminationReencrypt:
		//passthrough term should not specify any cert
	case routev1.TLSTerminationPassthrough:
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
	case routev1.TLSTerminationEdge:
		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, field.Invalid(fldPath.Child("destinationCACertificate"), "redacted destination ca certificate data", "edge termination does not support destination certificates"))
		}
	default:
		validValues := []string{string(routev1.TLSTerminationEdge), string(routev1.TLSTerminationPassthrough), string(routev1.TLSTerminationReencrypt)}
		result = append(result, field.NotSupported(fldPath.Child("termination"), tls.Termination, validValues))
	}

	if err := validateInsecureEdgeTerminationPolicy(tls, fldPath.Child("insecureEdgeTerminationPolicy")); err != nil {
		result = append(result, err)
	}

	return result
}

// validateInsecureEdgeTerminationPolicy tests fields for different types of
// insecure options. Called by validateTLS.
func validateInsecureEdgeTerminationPolicy(tls *routev1.TLSConfig, fldPath *field.Path) *field.Error {
	// Check insecure option value if specified (empty is ok).
	if len(tls.InsecureEdgeTerminationPolicy) == 0 {
		return nil
	}

	// It is an edge-terminated or reencrypt route, check insecure option value is
	// one of None(for disable), Allow or Redirect.
	allowedValues := map[routev1.InsecureEdgeTerminationPolicyType]struct{}{
		routev1.InsecureEdgeTerminationPolicyNone:     {},
		routev1.InsecureEdgeTerminationPolicyAllow:    {},
		routev1.InsecureEdgeTerminationPolicyRedirect: {},
	}

	switch tls.Termination {
	case routev1.TLSTerminationReencrypt:
		fallthrough
	case routev1.TLSTerminationEdge:
		if _, ok := allowedValues[tls.InsecureEdgeTerminationPolicy]; !ok {
			msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, %s, or empty", routev1.InsecureEdgeTerminationPolicyNone, routev1.InsecureEdgeTerminationPolicyAllow, routev1.InsecureEdgeTerminationPolicyRedirect)
			return field.Invalid(fldPath, tls.InsecureEdgeTerminationPolicy, msg)
		}
	case routev1.TLSTerminationPassthrough:
		if routev1.InsecureEdgeTerminationPolicyNone != tls.InsecureEdgeTerminationPolicy && routev1.InsecureEdgeTerminationPolicyRedirect != tls.InsecureEdgeTerminationPolicy {
			msg := fmt.Sprintf("invalid value for InsecureEdgeTerminationPolicy option, acceptable values are %s, %s, or empty", routev1.InsecureEdgeTerminationPolicyNone, routev1.InsecureEdgeTerminationPolicyRedirect)
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
