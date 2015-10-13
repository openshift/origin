package validation

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/pflag"

	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	kutil "k8s.io/kubernetes/pkg/util"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"

	"github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ValidateHostPort(value string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(value) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if _, _, err := net.SplitHostPort(value); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, value, "must be a host:port"))
	}

	return allErrs
}

func ValidateCertInfo(certInfo api.CertInfo, required bool) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if required || len(certInfo.CertFile) > 0 || len(certInfo.KeyFile) > 0 {
		if len(certInfo.CertFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("certFile"))
		}
		if len(certInfo.KeyFile) == 0 {
			allErrs = append(allErrs, fielderrors.NewFieldRequired("keyFile"))
		}
	}

	if len(certInfo.CertFile) > 0 {
		allErrs = append(allErrs, ValidateFile(certInfo.CertFile, "certFile")...)
	}

	if len(certInfo.KeyFile) > 0 {
		allErrs = append(allErrs, ValidateFile(certInfo.KeyFile, "keyFile")...)
	}

	// validate certfile/keyfile load/parse?

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateHostPort(info.BindAddress, "bindAddress")...)
	validationResults.AddErrors(ValidateCertInfo(info.ServerCert, false)...)

	if len(info.NamedCertificates) > 0 && len(info.ServerCert.CertFile) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("namedCertificates", "", "a default certificate and key is required in certFile/keyFile in order to use namedCertificates"))
	}

	validationResults.Append(ValidateNamedCertificates("namedCertificates", info.NamedCertificates))

	switch info.BindNetwork {
	case "tcp", "tcp4", "tcp6":
	default:
		validationResults.AddErrors(fielderrors.NewFieldInvalid("bindNetwork", info.BindNetwork, "must be 'tcp', 'tcp4', or 'tcp6'"))
	}

	if len(info.ServerCert.CertFile) > 0 {
		if len(info.ClientCA) > 0 {
			validationResults.AddErrors(ValidateFile(info.ClientCA, "clientCA")...)
		}
	} else {
		if len(info.ClientCA) > 0 {
			validationResults.AddErrors(fielderrors.NewFieldInvalid("clientCA", info.ClientCA, "cannot specify a clientCA without a certFile"))
		}
	}

	return validationResults
}

func ValidateNamedCertificates(fieldName string, namedCertificates []api.NamedCertificate) ValidationResults {
	validationResults := ValidationResults{}

	takenNames := sets.NewString()
	for i, namedCertificate := range namedCertificates {
		fieldName := fmt.Sprintf("%s[%d]", fieldName, i)

		certDNSNames := []string{}
		if len(namedCertificate.CertFile) == 0 {
			validationResults.AddErrors(fielderrors.NewFieldRequired(fieldName + ".certInfo"))
		} else if certInfoErrors := ValidateCertInfo(namedCertificate.CertInfo, false); len(certInfoErrors) > 0 {
			validationResults.AddErrors(certInfoErrors.Prefix(fieldName)...)
		} else if cert, err := tls.LoadX509KeyPair(namedCertificate.CertFile, namedCertificate.KeyFile); err != nil {
			validationResults.AddErrors(fielderrors.NewFieldInvalid(fieldName+".certInfo", namedCertificate.CertInfo, fmt.Sprintf("error loading certificate/key: %v", err)))
		} else {
			leaf, _ := x509.ParseCertificate(cert.Certificate[0])
			certDNSNames = append(certDNSNames, leaf.Subject.CommonName)
			certDNSNames = append(certDNSNames, leaf.DNSNames...)
		}

		if len(namedCertificate.Names) == 0 {
			validationResults.AddErrors(fielderrors.NewFieldRequired(fieldName + ".names"))
		}
		for j, name := range namedCertificate.Names {
			nameFieldName := fieldName + fmt.Sprintf(".names[%d]", j)
			if len(name) == 0 {
				validationResults.AddErrors(fielderrors.NewFieldRequired(nameFieldName))
				continue
			}

			if takenNames.Has(name) {
				validationResults.AddErrors(fielderrors.NewFieldInvalid(nameFieldName, name, "this name is already used in another named certificate"))
				continue
			}

			// validate names as domain names or *.*.foo.com domain names
			validDNSName := true
			for _, s := range strings.Split(name, ".") {
				if s != "*" && !kutil.IsDNS1123Label(s) {
					validDNSName = false
				}
			}
			if !validDNSName {
				validationResults.AddErrors(fielderrors.NewFieldInvalid(nameFieldName, name, "must be a valid DNS name"))
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
				}
				if !foundMatch {
					validationResults.AddWarnings(fielderrors.NewFieldInvalid(nameFieldName, name, "the specified certificate does not have a CommonName or DNS subjectAltName that matches this name"))
				}
			}
		}
	}

	return validationResults
}

func ValidateHTTPServingInfo(info api.HTTPServingInfo) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateServingInfo(info.ServingInfo))

	if info.MaxRequestsInFlight < 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("maxRequestsInFlight", info.MaxRequestsInFlight, "must be zero (no limit) or greater"))
	}

	if info.RequestTimeoutSeconds < -1 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("requestTimeoutSeconds", info.RequestTimeoutSeconds, "must be -1 (no timeout), 0 (default timeout), or greater"))
	}

	return validationResults
}

func ValidateDisabledFeatures(disabledFeatures []string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	for i, feature := range disabledFeatures {
		if _, isKnown := api.NormalizeOpenShiftFeature(feature); !isKnown {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(fmt.Sprintf("%s[%d]", field, i), disabledFeatures[i], fmt.Sprintf("not one of valid features: %s", strings.Join(api.KnownOpenShiftFeatures, ", "))))
		}
	}

	return allErrs
}

func ValidateKubeConfig(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateFile(path, field)...)
	// TODO: load and parse

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo api.RemoteConnectionInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("url"))
	} else {
		_, urlErrs := ValidateURL(remoteConnectionInfo.URL, "url")
		allErrs = append(allErrs, urlErrs...)
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(remoteConnectionInfo.CA, "ca")...)
	}

	allErrs = append(allErrs, ValidateCertInfo(remoteConnectionInfo.ClientCert, false)...)

	return allErrs
}

func ValidatePodManifestConfig(podManifestConfig *api.PodManifestConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// the Path can be a file or a directory
	allErrs = append(allErrs, ValidateFile(podManifestConfig.Path, "path")...)
	if podManifestConfig.FileCheckIntervalSeconds < 1 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid("fileCheckIntervalSeconds", podManifestConfig.FileCheckIntervalSeconds, "interval has to be positive"))
	}

	return allErrs
}

func ValidateSpecifiedIP(ipString string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	ip := net.ParseIP(ipString)
	if ip == nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "must be a valid IP"))
	} else if ip.IsUnspecified() {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, ipString, "cannot be an unspecified IP"))
	}

	return allErrs
}

func ValidateSecureURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	url, urlErrs := ValidateURL(urlString, field)
	if len(urlErrs) == 0 && url.Scheme != "https" {
		urlErrs = append(urlErrs, fielderrors.NewFieldInvalid(field, urlString, "must use https scheme"))
	}
	return url, urlErrs
}

func ValidateURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	allErrs := fielderrors.ValidationErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a scheme (e.g. https://)"))
	}
	if len(urlObj.Host) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a host"))
	}
	return urlObj, allErrs
}

func ValidateNamespace(namespace, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(namespace) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if ok, _ := kvalidation.ValidateNamespaceName(namespace, false); !ok {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, namespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateFile(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "could not read file"))
	}

	return allErrs
}

func ValidateDir(path string, field string) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}
	if len(path) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired(field))
	} else {
		fileInfo, err := os.Stat(path)
		if err != nil {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "could not read info"))
		} else if !fileInfo.IsDir() {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, path, "not a directory"))
		}
	}

	return allErrs
}

func ValidateExtendedArguments(config api.ExtendedArguments, flagFunc func(*pflag.FlagSet)) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	// check extended arguments for errors
	for _, err := range cmdflags.Resolve(config, flagFunc) {
		switch t := err.(type) {
		case *fielderrors.ValidationError:
			allErrs = append(allErrs, t)
		default:
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("????", config, err.Error()))
		}
	}

	return allErrs
}
