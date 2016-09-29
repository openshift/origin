package validation

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io/ioutil"
	"net"
	"net/url"
	"os"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/spf13/pflag"

	kvalidation "k8s.io/kubernetes/pkg/api/validation"
	"k8s.io/kubernetes/pkg/util/sets"
	utilvalidation "k8s.io/kubernetes/pkg/util/validation"
	"k8s.io/kubernetes/pkg/util/validation/field"

	"github.com/openshift/origin/pkg/cmd/server/api"
	cmdutil "github.com/openshift/origin/pkg/cmd/util"
	cmdflags "github.com/openshift/origin/pkg/cmd/util/flags"
)

func ValidateStringSource(s api.StringSource, fieldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}
	methods := 0
	if len(s.Value) > 0 {
		methods++
	}
	if len(s.File) > 0 {
		methods++
		fileErrors := ValidateFile(s.File, fieldPath.Child("file"))
		validationResults.AddErrors(fileErrors...)

		// If the file was otherwise ok, and its value will be used verbatim, warn about trailing whitespace
		if len(fileErrors) == 0 && len(s.KeyFile) == 0 {
			if data, err := ioutil.ReadFile(s.File); err != nil {
				validationResults.AddErrors(field.Invalid(fieldPath.Child("file"), s.File, fmt.Sprintf("could not read file: %v", err)))
			} else if len(data) > 0 {
				r, _ := utf8.DecodeLastRune(data)
				if unicode.IsSpace(r) {
					validationResults.AddWarnings(field.Invalid(fieldPath.Child("file"), s.File, "contains trailing whitespace which will be included in the value"))
				}
			}
		}
	}
	if len(s.Env) > 0 {
		methods++
	}
	if methods > 1 {
		validationResults.AddErrors(field.Invalid(fieldPath, "", "only one of value, file, and env can be specified"))
	}

	if len(s.KeyFile) > 0 {
		validationResults.AddErrors(ValidateFile(s.KeyFile, fieldPath.Child("keyFile"))...)
	}

	return validationResults
}

func ValidateHostPort(value string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(value) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else if _, _, err := net.SplitHostPort(value); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, value, "must be a host:port"))
	}

	return allErrs
}

func ValidateCertInfo(certInfo api.CertInfo, required bool, fldPath *field.Path) field.ErrorList {
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
		allErrs = append(allErrs, ValidateFile(certInfo.CertFile, fldPath.Child("certFile"))...)
	}

	if len(certInfo.KeyFile) > 0 {
		allErrs = append(allErrs, ValidateFile(certInfo.KeyFile, fldPath.Child("keyFile"))...)
	}

	// validate certfile/keyfile load/parse?

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.AddErrors(ValidateHostPort(info.BindAddress, fldPath.Child("bindAddress"))...)
	validationResults.AddErrors(ValidateCertInfo(info.ServerCert, false, fldPath)...)

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
			validationResults.AddErrors(ValidateFile(info.ClientCA, fldPath.Child("clientCA"))...)
		}
	} else {
		if len(info.ClientCA) > 0 {
			validationResults.AddErrors(field.Invalid(fldPath.Child("clientCA"), info.ClientCA, "cannot specify a clientCA without a certFile"))
		}
	}

	return validationResults
}

func ValidateNamedCertificates(fldPath *field.Path, namedCertificates []api.NamedCertificate) ValidationResults {
	validationResults := ValidationResults{}

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

func ValidateHTTPServingInfo(info api.HTTPServingInfo, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateServingInfo(info.ServingInfo, fldPath))

	if info.MaxRequestsInFlight < 0 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("maxRequestsInFlight"), info.MaxRequestsInFlight, "must be zero (no limit) or greater"))
	}

	if info.RequestTimeoutSeconds < -1 {
		validationResults.AddErrors(field.Invalid(fldPath.Child("requestTimeoutSeconds"), info.RequestTimeoutSeconds, "must be -1 (no timeout), 0 (default timeout), or greater"))
	}

	return validationResults
}

func ValidateDisabledFeatures(disabledFeatures []string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	for i, feature := range disabledFeatures {
		if _, isKnown := api.NormalizeOpenShiftFeature(feature); !isKnown {
			allErrs = append(allErrs, field.Invalid(fldPath.Index(i), disabledFeatures[i], fmt.Sprintf("not one of valid features: %s", strings.Join(api.KnownOpenShiftFeatures, ", "))))
		}
	}

	return allErrs
}

func ValidateKubeConfig(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	allErrs = append(allErrs, ValidateFile(path, fldPath)...)
	// TODO: load and parse

	return allErrs
}

func ValidateRemoteConnectionInfo(remoteConnectionInfo api.RemoteConnectionInfo, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(remoteConnectionInfo.URL) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("url"), ""))
	} else {
		_, urlErrs := ValidateURL(remoteConnectionInfo.URL, fldPath.Child("url"))
		allErrs = append(allErrs, urlErrs...)
	}

	if len(remoteConnectionInfo.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(remoteConnectionInfo.CA, fldPath.Child("ca"))...)
	}

	allErrs = append(allErrs, ValidateCertInfo(remoteConnectionInfo.ClientCert, false, fldPath)...)

	return allErrs
}

func ValidatePodManifestConfig(podManifestConfig *api.PodManifestConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	// the Path can be a file or a directory
	allErrs = append(allErrs, ValidateFile(podManifestConfig.Path, fldPath.Child("path"))...)
	if podManifestConfig.FileCheckIntervalSeconds < 1 {
		allErrs = append(allErrs, field.Invalid(fldPath.Child("fileCheckIntervalSeconds"), podManifestConfig.FileCheckIntervalSeconds, "interval has to be positive"))
	}

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

func ValidateSecureURL(urlString string, fldPath *field.Path) (*url.URL, field.ErrorList) {
	url, urlErrs := ValidateURL(urlString, fldPath)
	if len(urlErrs) == 0 && url.Scheme != "https" {
		urlErrs = append(urlErrs, field.Invalid(fldPath, urlString, "must use https scheme"))
	}
	return url, urlErrs
}

func ValidateURL(urlString string, fldPath *field.Path) (*url.URL, field.ErrorList) {
	allErrs := field.ErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must contain a scheme (e.g. https://)"))
	}
	if len(urlObj.Host) == 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, urlString, "must contain a host"))
	}
	return urlObj, allErrs
}

func ValidateNamespace(namespace string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(namespace) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else if reasons := kvalidation.ValidateNamespaceName(namespace, false); len(reasons) != 0 {
		allErrs = append(allErrs, field.Invalid(fldPath, namespace, "must be a valid namespace"))
	}

	return allErrs
}

func ValidateFile(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(path) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else if _, err := os.Stat(path); err != nil {
		allErrs = append(allErrs, field.Invalid(fldPath, path, "could not read file"))
	}

	return allErrs
}

func ValidateDir(path string, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}
	if len(path) == 0 {
		allErrs = append(allErrs, field.Required(fldPath, ""))
	} else {
		fileInfo, err := os.Stat(path)
		if err != nil {
			allErrs = append(allErrs, field.Invalid(fldPath, path, "could not read info"))
		} else if !fileInfo.IsDir() {
			allErrs = append(allErrs, field.Invalid(fldPath, path, "not a directory"))
		}
	}

	return allErrs
}

func ValidateExtendedArguments(config api.ExtendedArguments, flagFunc func(*pflag.FlagSet), fldPath *field.Path) field.ErrorList {
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
