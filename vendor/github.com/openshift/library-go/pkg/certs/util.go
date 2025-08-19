package certs

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"
)

const defaultOutputTimeFormat = "Jan 2 15:04:05 2006"

// nowFn is used in unit test to freeze time.
var nowFn = time.Now().UTC

// CertificateToString converts a certificate into a human readable string.
// This function should guarantee consistent output format for must-gather tooling and any code
// that prints the certificate details.
func CertificateToString(certificate *x509.Certificate) string {
	humanName := certificate.Subject.CommonName
	signerHumanName := certificate.Issuer.CommonName

	if certificate.Subject.CommonName == certificate.Issuer.CommonName {
		signerHumanName = "<self-signed>"
	}

	usages := []string{}
	for _, curr := range certificate.ExtKeyUsage {
		if curr == x509.ExtKeyUsageClientAuth {
			usages = append(usages, "client")
			continue
		}
		if curr == x509.ExtKeyUsageServerAuth {
			usages = append(usages, "serving")
			continue
		}

		usages = append(usages, fmt.Sprintf("%d", curr))
	}

	validServingNames := []string{}
	for _, ip := range certificate.IPAddresses {
		validServingNames = append(validServingNames, ip.String())
	}
	for _, dnsName := range certificate.DNSNames {
		validServingNames = append(validServingNames, dnsName)
	}

	servingString := ""
	if len(validServingNames) > 0 {
		servingString = fmt.Sprintf(" validServingFor=[%s]", strings.Join(validServingNames, ","))
	}

	groupString := ""
	if len(certificate.Subject.Organization) > 0 {
		groupString = fmt.Sprintf(" groups=[%s]", strings.Join(certificate.Subject.Organization, ","))
	}

	return fmt.Sprintf("%q [%s]%s%s issuer=%q (%v to %v (now=%v))", humanName, strings.Join(usages, ","), groupString,
		servingString, signerHumanName, certificate.NotBefore.UTC().Format(defaultOutputTimeFormat),
		certificate.NotAfter.UTC().Format(defaultOutputTimeFormat), nowFn().Format(defaultOutputTimeFormat))
}

// CertificateBundleToString converts a certificate bundle into a human readable string.
func CertificateBundleToString(bundle []*x509.Certificate) string {
	output := []string{}
	for i, cert := range bundle {
		output = append(output, fmt.Sprintf("[#%d]: %s", i, CertificateToString(cert)))
	}
	return strings.Join(output, "\n")
}
