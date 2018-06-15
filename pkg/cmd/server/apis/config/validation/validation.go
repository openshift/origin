package validation

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strings"

	"github.com/spf13/pflag"

	"k8s.io/apimachinery/pkg/util/sets"
	utilvalidation "k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/apimachinery/pkg/util/validation/field"

	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/cmd/server/apis/config/validation/common"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ValidateHostPort(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(value) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else if _, _, err := net.SplitHostPort(value); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, value, "must be a host:port"))
	}

	return allErrs
}

func ValidateCertInfo(certInfo config.CertInfo, required bool, fldPath *field.Path) field.ErrorList {
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
		allErrs = append(allErrs, common.ValidateFile(certInfo.CertFile, fldPath.Child("certFile"))...)
	}

	if len(certInfo.KeyFile) > 0 {
		allErrs = append(allErrs, common.ValidateFile(certInfo.KeyFile, fldPath.Child("keyFile"))...)
	}

	// validate certfile/keyfile load/parse?

	return allErrs
}

func ValidateServingInfo(info config.ServingInfo, certificatesRequired bool, fldPath *field.Path) common.ValidationResults {
	validationResults := common.ValidationResults{}

	validationResults.AddErrors(ValidateHostPort(info.BindAddress, fldPath.Child("bindAddress"))...)
	validationResults.AddErrors(ValidateCertInfo(info.ServerCert, certificatesRequired, fldPath)...)

	if len(info.NamedCertificates) > 0 && len(info.ServerCert.CertFile) == 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("namedCertificates"), "", "a default certificate and key is required in certFile/keyFile in order to use namedCertificates"))
	}

	validationResults.Append(ValidateNamedCertificates(fldPath.Child("namedCertificates"), info.NamedCertificates))

	switch info.BindNetwork {
	case "tcp", "tcp4", "tcp6":
	default:
		validationResults.AddErrors(field.Invalid(fldPath.Child("bindNetwork"), info.BindNetwork, "must be 'tcp', 'tcp4', or 'tcp6'"))
	}

	if len(info.ServerCert.CertFile) > 0 {
		if len(info.ClientCA) > 0 {
			validationResults.AddErrors(common.ValidateFile(info.ClientCA, fldPath.Child("clientCA"))...)
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

func ValidateNamedCertificates(fldPath *field.Path, namedCertificates []config.NamedCertificate) common.ValidationResults {
	validationResults := common.ValidationResults{}

	takenNames := sets.NewString()
	for i, namedCertificate := range namedCertificates {
		idxPath := fldPath.Index(i)

		certDNSNames := []string{}
		if len(namedCertificate.CertFile) == 0 {
			validationResults.AddErrors(field.Required(idxPath.Child("certInfo"), ""))
		} else if certInfoErrors := ValidateCertInfo(namedCertificate.CertInfo, false, idxPath); len(certInfoErrors) > 0 {
			validationResults.AddErrors(certInfoErrors...)
		} else if cert, err := tls.LoadX509KeyPair(namedCertificate.CertFile, namedCertificate.KeyFile); err != nil {
			validationResults.AddErrors(field.Invalid(idxPath.Child("certInfo"), namedCertificate.CertInfo, fmt.Sprintf("error loading certificate/key: %v", err)))
		} else {
			leaf, _ := x509.ParseCertificate(cert.Certificate[0])
			certDNSNames = append(certDNSNames, leaf.Subject.CommonName)
			certDNSNames = append(certDNSNames, leaf.DNSNames...)
		}

		if len(namedCertificate.Names) == 0 {
			validationResults.AddErrors(field.Required(idxPath.Child("names"), ""))
		}
		for j, name := range namedCertificate.Names {
			jdxPath := idxPath.Child("names").Index(j)
			if len(name) == 0 {
				validationResults.AddErrors(field.Required(jdxPath, ""))
				continue
			}

			if takenNames.Has(name) {
				validationResults.AddErrors(field.Invalid(jdxPath, name, "this name is already used in another named certificate"))
				continue
			}

			// validate names as domain names or *.*.foo.com domain names
			validDNSName := true
			for _, s := range strings.Split(name, ".") {
				if s != "*" && len(utilvalidation.IsDNS1123Label(s)) != 0 {
					validDNSName = false
				}
			}
			if !validDNSName {
				validationResults.AddErrors(field.Invalid(jdxPath, name, "must be a valid DNS name"))
				continue
			}

			takenNames.Insert(name)

			// validate certificate has common name or subject alt names that match
			if len(certDNSNames) > 0 {
				foundMatch := false
				for _, dnsName := range certDNSNames {
					if cmdutil.HostnameMatches(dnsName, name) {
						foundMatch = true
						break
					}
					// if the cert has a wildcard dnsName, and we've configured a non-wildcard name, see if our specified name will match against the dnsName.
					if strings.HasPrefix(dnsName, "*.") && !strings.HasPrefix(name, "*.") && cmdutil.HostnameMatches(name, dnsName) {
						foundMatch = true
						break
					}
				}
				if !foundMatch {
					validationResults.AddWarnings(field.Invalid(jdxPath, name, "the specified certificate does not have a CommonName or DNS subjectAltName that matches this name"))
				}
			}
		}
	}

	return validationResults
}

func ValidateHTTPServingInfo(info config.HTTPServingInfo, fldPath *field.Path) common.ValidationResults {
	validationResults := common.ValidationResults{}

	validationResults.Append(ValidateServingInfo(info.ServingInfo, true, fldPath))

	if info.MaxRequestsInFlight < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maxRequestsInFlight"), info.MaxRequestsInFlight, "must be zero (no limit) or greater"))
	}

	if info.RequestTimeoutSeconds < -1 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("requestTimeoutSeconds"), info.RequestTimeoutSeconds, "must be -1 (no timeout), 0 (default timeout), or greater"))
	}

	return validationResults
}

func ValidateKubeConfig(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, common.ValidateFile(path, fldPath)...)
	// TODO: load and parse

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo config.RemoteConnectionInfo, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), ""))
	} else {
		_, urlErrs := common.ValidateURL(remoteConnectionInfo.URL, fldPath.Child("url"))
		allErrs = append(allErrs, urlErrs...)
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, common.ValidateFile(remoteConnectionInfo.CA, fldPath.Child("ca"))...)
	}

	allErrs = append(allErrs, ValidateCertInfo(remoteConnectionInfo.ClientCert, false, fldPath)...)

	return allErrs
}

func ValidatePodManifestConfig(podManifestConfig *config.PodManifestConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// the Path can be a file or a directory
	allErrs = append(allErrs, common.ValidateFile(podManifestConfig.Path, fldPath.Child("path"))...)
	if podManifestConfig.FileCheckIntervalSeconds < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("fileCheckIntervalSeconds"), podManifestConfig.FileCheckIntervalSeconds, "interval has to be positive"))
	}

	return allErrs
}

func ValidateExtendedArguments(config config.ExtendedArguments, flagFunc func(*pflag.FlagSet), fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// check extended arguments for errors
	for _, err := range cmdflags.Resolve(config, flagFunc) {
		switch t := err.(type) {
		case *field.Error:
			allErrs = append(allErrs, t)
		default:
			allErrs = append(allErrs, field.Invalid(fldPath.Child("????"), config, err.Error()))
		}
	}

	return allErrs
}
