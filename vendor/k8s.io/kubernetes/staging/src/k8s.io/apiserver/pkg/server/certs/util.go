package certs

import (
	"crypto/x509"
	"fmt"
	"strings"
	"time"
)

func getCertDetail(certificate *x509.Certificate) string {
	humanName := certificate.Subject.CommonName
	signerHumanName := certificate.Issuer.CommonName
	if certificate.Subject.CommonName == certificate.Issuer.CommonName {
		signerHumanName = "<self>"
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

	return fmt.Sprintf("%q [%s]%s%s issuer=%q (%v to %v (now=%v))", humanName, strings.Join(usages, ","), groupString, servingString, signerHumanName, certificate.NotBefore.UTC(), certificate.NotAfter.UTC(),
		time.Now().UTC())
}
