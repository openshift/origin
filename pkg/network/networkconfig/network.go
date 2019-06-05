package networkconfig

import (
	"fmt"
	"net"
	"time"

	"k8s.io/apimachinery/pkg/util/validation/field"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/validation"
	"github.com/openshift/library-go/pkg/crypto"
)

func ValidateInClusterNetworkNodeConfig(config *legacyconfigv1.NodeConfig, fldPath *field.Path) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

	hasBootstrapConfig := len(config.KubeletArguments["bootstrap-kubeconfig"]) > 0
	if len(config.NodeName) == 0 && !hasBootstrapConfig {
		validationResults.AddErrors(field.Required(fldPath.Child("nodeName"), ""))
	}
	if len(config.NodeIP) > 0 {
		validationResults.AddErrors(ValidateSpecifiedIP(config.NodeIP, fldPath.Child("nodeIP"))...)
	}

	servingInfoPath := fldPath.Child("servingInfo")
	validationResults.Append(ValidateServingInfo(config.ServingInfo, false, servingInfoPath))

	validationResults.AddErrors(ValidateNetworkConfig(config.NetworkConfig, fldPath.Child("networkConfig"))...)

	if _, err := time.ParseDuration(config.IPTablesSyncPeriod); err != nil {
		validationResults.AddErrors(field.Invalid(fldPath.Child("iptablesSyncPeriod"), config.IPTablesSyncPeriod, fmt.Sprintf("unable to parse iptablesSyncPeriod: %v. Examples with correct format: '5s', '1m', '2h22m'", err)))
	}

	return validationResults
}

func ValidateNetworkConfig(config legacyconfigv1.NodeNetworkConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if config.MTU == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("mtu"), config.MTU, fmt.Sprintf("must be greater than zero")))
	}
	return allErrs
}

func ValidateServingInfo(info legacyconfigv1.ServingInfo, certificatesRequired bool, fldPath *field.Path) validation.ValidationResults {
	validationResults := validation.ValidationResults{}

	validationResults.AddErrors(validation.ValidateHostPort(info.BindAddress, fldPath.Child("bindAddress"))...)
	validationResults.AddErrors(ValidateCertInfo(info.CertInfo, certificatesRequired, fldPath)...)

	if len(info.NamedCertificates) > 0 && len(info.CertInfo.CertFile) == 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("namedCertificates"), "", "a default certificate and key is required in certFile/keyFile in order to use namedCertificates"))
	}

	if len(info.CertInfo.CertFile) > 0 {
		if len(info.ClientCA) > 0 {
			validationResults.AddErrors(validation.ValidateFile(info.ClientCA, fldPath.Child("clientCA"))...)
		}
	} else {
		if certificatesRequired && len(info.ClientCA) > 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("clientCA"), info.ClientCA, "cannot specify a clientCA without a certFile"))
		}
	}

	if _, err := crypto.TLSVersion(info.MinTLSVersion); err != nil {
		validationResults.AddErrors(field.NotSupported(fldPath.Child("minTLSVersion"), info.MinTLSVersion, crypto.ValidTLSVersions()))
	}
	for i, cipher := range info.CipherSuites {
		if _, err := crypto.CipherSuite(cipher); err != nil {
			validationResults.AddErrors(field.NotSupported(fldPath.Child("cipherSuites").Index(i), cipher, crypto.ValidCipherSuites()))
		}
	}

	return validationResults
}

func ValidateCertInfo(certInfo legacyconfigv1.CertInfo, required bool, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if required {
		if len(certInfo.CertFile) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("certFile"), "The certificate file must be provided"))
		}
		if len(certInfo.KeyFile) == 0 {
			allErrs = append(allErrs, field.Required(fldPath.Child("keyFile"), "The certificate key must be provided"))
		}
	}

	if (len(certInfo.CertFile) == 0) != (len(certInfo.KeyFile) == 0) {
		allErrs = append(allErrs, field.Required(fldPath.Child("certFile"), "Both the certificate file and the certificate key must be provided together or not at all"))
		allErrs = append(allErrs, field.Required(fldPath.Child("keyFile"), "Both the certificate file and the certificate key must be provided together or not at all"))
	}

	if len(certInfo.CertFile) > 0 {
		allErrs = append(allErrs, validation.ValidateFile(certInfo.CertFile, fldPath.Child("certFile"))...)
	}

	if len(certInfo.KeyFile) > 0 {
		allErrs = append(allErrs, validation.ValidateFile(certInfo.KeyFile, fldPath.Child("keyFile"))...)
	}

	// validate certfile/keyfile load/parse?

	return allErrs
}

func ValidateSpecifiedIP(ipString string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, field.Invalid(fldPath, ipString, "cannot be an unspecified IP"))
	}

	return allErrs
}
