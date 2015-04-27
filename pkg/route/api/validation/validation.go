package validation

import (
	ctls "crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"strings"

	kval "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	routeapi "github.com/openshift/origin/pkg/route/api"
)

// ValidateRoute tests if required fields in the route are set.
func ValidateRoute(route *routeapi.Route) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	//ensure meta is set properly
	result = append(result, kval.ValidateObjectMeta(&route.ObjectMeta, true, kval.ValidatePodName)...)

	//host is not required but if it is set ensure it meets DNS requirements
	if len(route.Host) > 0 {
		if !util.IsDNS1123Subdomain(route.Host) {
			result = append(result, fielderrors.NewFieldInvalid("host", route.Host, "Host must conform to DNS 952 subdomain conventions"))
		}
	}

	if len(route.Path) > 0 && !strings.HasPrefix(route.Path, "/") {
		result = append(result, fielderrors.NewFieldInvalid("path", route.Path, "Path must begin with /"))
	}

	if len(route.ServiceName) == 0 {
		result = append(result, fielderrors.NewFieldRequired("serviceName"))
	}

	if errs := validateTLS(route.TLS); len(errs) != 0 {
		result = append(result, errs.Prefix("tls")...)
	}

	return result
}

// ValidateTLS tests fields for different types of TLS combinations are set.  Called
// by ValidateRoute.
func validateTLS(tls *routeapi.TLSConfig) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}

	//no termination, ignore other settings
	if tls == nil || tls.Termination == "" {
		return nil
	}

	switch tls.Termination {
	//reencrypt must specify cert, key, cacert, and destination ca cert
	case routeapi.TLSTerminationReencrypt:
		if len(tls.Certificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("certificate"))
		}

		if len(tls.Key) == 0 {
			result = append(result, fielderrors.NewFieldRequired("key"))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("caCertificate"))
		}

		if len(tls.DestinationCACertificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("destinationCACertificate"))
		}

		result = append(result, validateTLSCertificates(tls)...)

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
	//edge cert should specify cert, key, and cacert
	case routeapi.TLSTerminationEdge:
		if len(tls.Certificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("certificate"))
		}

		if len(tls.Key) == 0 {
			result = append(result, fielderrors.NewFieldRequired("key"))
		}

		if len(tls.CACertificate) == 0 {
			result = append(result, fielderrors.NewFieldRequired("caCertificate"))
		}

		if len(tls.DestinationCACertificate) > 0 {
			result = append(result, fielderrors.NewFieldInvalid("destinationCACertificate", tls.DestinationCACertificate, "edge termination does not support destination certificates"))
		}

		result = append(result, validateTLSCertificates(tls)...)

	default:
		msg := fmt.Sprintf("invalid value for termination, acceptable values are %s, %s, %s, or emtpy (no tls specified)", routeapi.TLSTerminationEdge, routeapi.TLSTerminationPassthrough, routeapi.TLSTerminationReencrypt)
		result = append(result, fielderrors.NewFieldInvalid("termination", tls.Termination, msg))
	}
	return result
}

// validateTLSCertificates checks that the certificates passed are able to be parsed.  This includes ensuring that
// the cert/key pair are matching and, if a ca cert is provided it will check the first ca cert against the leaf
// to ensure that the leaf was signed by the ca
func validateTLSCertificates(tls *routeapi.TLSConfig) fielderrors.ValidationErrorList {
	if tls == nil {
		return nil
	}
	//edge and reencrypt (the use cases that call this require all 3 (at least) so we can ignore anything
	//that doesn't have all 3 set.  It will already fail validation elsewhere
	if len(tls.Certificate) == 0 || len(tls.Key) == 0 || len(tls.CACertificate) == 0 {
		return nil
	}

	result := fielderrors.ValidationErrorList{}

	//check cert/key pair
	checkSignature := true
	cert, err := ctls.X509KeyPair([]byte(decodeNewlines(tls.Certificate)), []byte(decodeNewlines(tls.Key)))
	if err != nil {
		msg := fmt.Sprintf("the certificate and key were not able to be parsed: %s", err.Error())
		//no value set on this, it is a multi-field validation
		result = append(result, fielderrors.NewFieldInvalid("certificate/key", "", msg))
		//since we didn't parse the cert correctly we should skip the signature check in the ca validations
		checkSignature = false
	}
	//cert was parsed, check that the ca can be parsed and has signed the cert if given
	result = append(result, validateCACertBlock(decodeNewlines(tls.CACertificate), "caCertificate", checkSignature, &cert)...)
	//check that the dest cert can be parsed if given
	result = append(result, validateCACertBlock(decodeNewlines(tls.DestinationCACertificate), "destinationCACertificate", false, &cert)...)
	return result
}

// validateCACertBlock iterates through all certs in a ca cert block to ensure that they can be parsed.  Optionally,
// it checks that a ca cert has signed the passed in cert.  This checkSignature is used by the caCertificate check but not the
// destinationCertificate check
func validateCACertBlock(caCert, field string, checkSignature bool, cert *ctls.Certificate) fielderrors.ValidationErrorList {
	result := fielderrors.ValidationErrorList{}
	if len(caCert) == 0 {
		return result
	}

	//decode and parse the ca cert.  When a pem block is decoded the following occurs:
	//1. if a block is parsed it is put into block
	//2. if a block is parsed then the remaining data will be in rest
	//3. if the data is not PEM data then block will be nil and the data will be in rest
	block, rest := pem.Decode([]byte(caCert))
	var parsedCACert *x509.Certificate
	var err error

	//not pem data
	if block == nil {
		result = append(result, fielderrors.NewFieldInvalid(field, caCert, "unable to parse certificate"))
	} else {
		//pem data found
		//keep the first ca cert so we can check signatures if requested
		parsedCACert, err = x509.ParseCertificate(block.Bytes)
		//bad CA, don't keep parsing the chain and just return
		if err != nil {
			msg := fmt.Sprintf("the %s certificate was not able to be parsed: %s", field, err.Error())
			result = append(result, fielderrors.NewFieldInvalid(field, caCert, msg))
			return result
		}

		//check remaining rest data for more pem blocks (a cert chain concatenated in the field)
		if len(rest) > 0 {
			for {
				block, rest = pem.Decode(rest)
				//not pem data
				if block == nil {
					result = append(result, fielderrors.NewFieldInvalid(field, caCert, "unable to parse certificate chain"))
					break
				}
				//check the cert
				_, err := x509.ParseCertificate(block.Bytes)
				if err != nil {
					msg := fmt.Sprintf("the %s certificate chain was not able to be parsed: %s", field, err.Error())
					result = append(result, fielderrors.NewFieldInvalid(field, caCert, msg))
				}
				//check if we're done with the pem blocks now
				if len(rest) == 0 {
					break
				}
			}
		}
	}

	if checkSignature && parsedCACert != nil {
		//load up the leaf cert as an x509 certificate so we can check the signatures
		//this shouldn't die since we were able to parse it in validateTLSCertificates and tls.X509KeyPair performs this same
		//check, it just doesn't set the .Leaf value.
		leaf, err := x509.ParseCertificate(cert.Certificate[0])
		if err != nil {
			msg := fmt.Sprintf("the certificate was not able to be parsed %s", err.Error())
			//no value set on this because at this time we have a decoded pem block, not what was originally submitted
			result = append(result, fielderrors.NewFieldInvalid("certificate", "", msg))
			return result
		}

		err = leaf.CheckSignatureFrom(parsedCACert)
		if err != nil {
			msg := fmt.Sprintf("invalid ca cert: %s", err.Error())
			//no value set because it is a multi-field validation
			result = append(result, fielderrors.NewFieldInvalid("certificate/caCertificate", "", msg))
		}
	}

	return result
}

// decodeNewlines is utility to remove the json formatted newlines from a cert
func decodeNewlines(s string) string {
	return strings.Replace(s, "\\n", "\n", -1)
}
