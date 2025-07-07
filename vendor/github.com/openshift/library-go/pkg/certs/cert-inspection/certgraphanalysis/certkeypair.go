package certgraphanalysis

import (
	"crypto/ecdsa"
	"crypto/rsa"
	"crypto/x509"
	"fmt"
	"os"
	"time"

	"github.com/openshift/library-go/pkg/certs/cert-inspection/certgraphapi"
	"k8s.io/apimachinery/pkg/util/duration"
)

func toCertKeyPair(certificate *x509.Certificate) (*certgraphapi.CertKeyPair, error) {
	ret := &certgraphapi.CertKeyPair{
		Name: fmt.Sprintf("%v::%v", certificate.Subject.CommonName, certificate.SerialNumber),
		Spec: certgraphapi.CertKeyPairSpec{
			SecretLocations: nil,
			OnDiskLocations: nil,
			CertMetadata:    toCertKeyMetadata(certificate),
		},
	}

	details, err := toCertKeyPairDetails(certificate)
	ret.Spec.Details = details
	if err != nil {
		ret.Status.Errors = append(ret.Status.Errors, err.Error())
	}

	return ret, nil
}

func toCertKeyPairDetails(certificate *x509.Certificate) (certgraphapi.CertKeyPairDetails, error) {
	usageCount := 0
	isClient := false
	isServing := false
	isSigner := (certificate.KeyUsage & x509.KeyUsageCertSign) != 0
	if isSigner {
		usageCount++
	}
	for _, curr := range certificate.ExtKeyUsage {
		if curr == x509.ExtKeyUsageClientAuth {
			isClient = true
			usageCount++
		}
		if curr == x509.ExtKeyUsageServerAuth {
			isServing = true
			usageCount++
		}
	}
	var typeError error
	if usageCount == 0 {
		typeError = fmt.Errorf("you have a cert for nothing?")
	}
	if usageCount > 1 {
		typeError = fmt.Errorf("you have a cert for more than one?  We don't do that. :(")
	}

	ret := certgraphapi.CertKeyPairDetails{}
	if isClient {
		ret.CertType = "ClientCertDetails"
		ret.ClientCertDetails = toClientCertDetails(certificate)
	}
	if isServing {
		ret.CertType = "ServingCertDetails"
		ret.ServingCertDetails = toServingCertDetails(certificate)
	}
	if isSigner {
		ret.CertType = "SignerCertDetails"
		ret.SignerDetails = toSignerDetails(certificate)
	}

	if usageCount > 1 {
		ret.CertType = "Multiple"
	}

	return ret, typeError
}

func toClientCertDetails(certificate *x509.Certificate) *certgraphapi.ClientCertDetails {
	return &certgraphapi.ClientCertDetails{
		Organizations: certificate.Subject.Organization,
	}
}

func toServingCertDetails(certificate *x509.Certificate) *certgraphapi.ServingCertDetails {
	ret := &certgraphapi.ServingCertDetails{
		DNSNames:    certificate.DNSNames,
		IPAddresses: nil,
	}

	for _, curr := range certificate.IPAddresses {
		ret.IPAddresses = append(ret.IPAddresses, curr.String())
	}

	return ret
}

func toSignerDetails(certificate *x509.Certificate) *certgraphapi.SignerCertDetails {
	return &certgraphapi.SignerCertDetails{}
}

func toCertKeyMetadata(certificate *x509.Certificate) certgraphapi.CertKeyMetadata {
	ret := certgraphapi.CertKeyMetadata{
		CertIdentifier: certgraphapi.CertIdentifier{
			CommonName:   certificate.Subject.CommonName,
			SerialNumber: certificate.SerialNumber.String(),
		},
		SignatureAlgorithm: certificate.SignatureAlgorithm.String(),
		PublicKeyAlgorithm: certificate.PublicKeyAlgorithm.String(),
		NotBefore:          certificate.NotBefore.Format(time.RFC3339),
		NotAfter:           certificate.NotAfter.Format(time.RFC3339),
		ValidityDuration:   duration.HumanDuration(certificate.NotAfter.Sub(certificate.NotBefore)),
	}

	switch publicKey := certificate.PublicKey.(type) {
	case *ecdsa.PublicKey:
		ret.PublicKeyBitSize = fmt.Sprintf("%d bit, %v curve", publicKey.Params().BitSize, publicKey.Params().Name)
		ret.CertIdentifier.PubkeyModulus = fmt.Sprintf("%s %s", publicKey.X.String(), publicKey.Y.String())
	case *rsa.PublicKey:
		ret.PublicKeyBitSize = fmt.Sprintf("%d bit", publicKey.Size()*8)
		ret.CertIdentifier.PubkeyModulus = publicKey.N.String()
	default:
		fmt.Fprintf(os.Stderr, "%T\n", publicKey)
	}

	signerHumanName := certificate.Issuer.CommonName
	ret.CertIdentifier.Issuer = &certgraphapi.CertIdentifier{
		CommonName:   signerHumanName,
		SerialNumber: certificate.Issuer.SerialNumber,
	}

	humanUsages := []string{}
	if (certificate.KeyUsage & x509.KeyUsageDigitalSignature) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDigitalSignature")
	}
	if (certificate.KeyUsage & x509.KeyUsageContentCommitment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageContentCommitment")
	}
	if (certificate.KeyUsage & x509.KeyUsageKeyEncipherment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageKeyEncipherment")
	}
	if (certificate.KeyUsage & x509.KeyUsageDataEncipherment) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDataEncipherment")
	}
	if (certificate.KeyUsage & x509.KeyUsageKeyAgreement) != 0 {
		humanUsages = append(humanUsages, "KeyUsageKeyAgreement")
	}
	if (certificate.KeyUsage & x509.KeyUsageCertSign) != 0 {
		humanUsages = append(humanUsages, "KeyUsageCertSign")
	}
	if (certificate.KeyUsage & x509.KeyUsageCRLSign) != 0 {
		humanUsages = append(humanUsages, "KeyUsageCRLSign")
	}
	if (certificate.KeyUsage & x509.KeyUsageEncipherOnly) != 0 {
		humanUsages = append(humanUsages, "KeyUsageEncipherOnly")
	}
	if (certificate.KeyUsage & x509.KeyUsageDecipherOnly) != 0 {
		humanUsages = append(humanUsages, "KeyUsageDecipherOnly")
	}
	ret.Usages = humanUsages

	humanExtendedUsages := []string{}
	for _, curr := range certificate.ExtKeyUsage {
		switch curr {
		case x509.ExtKeyUsageAny:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageAny")
		case x509.ExtKeyUsageServerAuth:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageServerAuth")
		case x509.ExtKeyUsageClientAuth:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageClientAuth")
		case x509.ExtKeyUsageCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageCodeSigning")
		case x509.ExtKeyUsageEmailProtection:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageEmailProtection")
		case x509.ExtKeyUsageIPSECEndSystem:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECEndSystem")
		case x509.ExtKeyUsageIPSECTunnel:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECTunnel")
		case x509.ExtKeyUsageIPSECUser:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageIPSECUser")
		case x509.ExtKeyUsageTimeStamping:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageTimeStamping")
		case x509.ExtKeyUsageOCSPSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageOCSPSigning")
		case x509.ExtKeyUsageMicrosoftServerGatedCrypto:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftServerGatedCrypto")
		case x509.ExtKeyUsageNetscapeServerGatedCrypto:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageNetscapeServerGatedCrypto")
		case x509.ExtKeyUsageMicrosoftCommercialCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftCommercialCodeSigning")
		case x509.ExtKeyUsageMicrosoftKernelCodeSigning:
			humanExtendedUsages = append(humanExtendedUsages, "ExtKeyUsageMicrosoftKernelCodeSigning")
		default:
			panic(fmt.Sprintf("unrecognized %v", curr))
		}
	}
	ret.ExtendedUsages = humanExtendedUsages

	return ret
}

func addSecretLocation(in *certgraphapi.CertKeyPair, namespace, name string) *certgraphapi.CertKeyPair {
	secretLocation := certgraphapi.InClusterSecretLocation{
		Namespace: namespace,
		Name:      name,
	}
	out := in.DeepCopy()
	for _, curr := range in.Spec.SecretLocations {
		if curr == secretLocation {
			return out
		}
	}

	out.Spec.SecretLocations = append(out.Spec.SecretLocations, secretLocation)
	return out
}
