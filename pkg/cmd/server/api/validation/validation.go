package validation

import (
	"net"
	"net/url"
	"os"

	kvalidation "github.com/GoogleCloudPlatform/kubernetes/pkg/api/validation"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util/fielderrors"

	"github.com/openshift/origin/pkg/cmd/server/api"
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

	return allErrs
}

func ValidateServingInfo(info api.ServingInfo) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	allErrs = append(allErrs, ValidateHostPort(info.BindAddress, "bindAddress")...)
	allErrs = append(allErrs, ValidateCertInfo(info.ServerCert, false)...)

	if len(info.ServerCert.CertFile) > 0 {
		if len(info.ClientCA) > 0 {
			allErrs = append(allErrs, ValidateFile(info.ClientCA, "clientCA")...)
		}
	} else {
		if len(info.ClientCA) > 0 {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("clientCA", info.ClientCA, "cannot specify a clientCA without a certFile"))
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

func ValidateURL(urlString string, field string) (*url.URL, fielderrors.ValidationErrorList) {
	allErrs := fielderrors.ValidationErrorList{}

	urlObj, err := url.Parse(urlString)
	if err != nil {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must be a valid URL"))
		return nil, allErrs
	}
	if len(urlObj.Scheme) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldInvalid(field, urlString, "must contain a scheme (e.g. http://)"))
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
