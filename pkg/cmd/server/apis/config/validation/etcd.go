package validation

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/cmd/server/apis/config"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/apimachinery/pkg/util/validation/field"
)

// ValidateEtcdConnectionInfo validates the connection info. If a server EtcdConfig is provided,
// it ensures the connection info includes a URL for it, and has a client cert/key if the server requires
// client certificate authentication
func ValidateEtcdConnectionInfo(config config.EtcdConnectionInfo, server *config.EtcdConfig, fldPath *field.Path) field.ErrorList {
	allErrs := field.ErrorList{}

	if len(config.URLs) == 0 {
		allErrs = append(allErrs, field.Required(fldPath.Child("urls"), ""))
	}
	for i, u := range config.URLs {
		_, urlErrs := ValidateURL(u, fldPath.Child("urls").Index(i))
		if len(urlErrs) > 0 {
			allErrs = append(allErrs, urlErrs...)
		}
	}

	if len(config.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(config.CA, fldPath.Child("ca"))...)
	}
	allErrs = append(allErrs, ValidateCertInfo(config.ClientCert, false, fldPath)...)

	// If we have server config info, make sure the client connection info will work with it
	if server != nil {
		var builtInAddress = fmt.Sprintf("https://%s", server.Address)

		// Require a client cert to connect to an etcd that requires client certs
		if len(server.ServingInfo.ClientCA) > 0 {
			if len(config.ClientCert.CertFile) == 0 {
				allErrs = append(allErrs, field.Required(fldPath.Child("certFile"), "A client certificate must be provided for this etcd server"))
			}
		}

		// Require the etcdClientInfo to include the address of the internal etcd
		clientURLs := sets.NewString(config.URLs...)
		if !clientURLs.Has(builtInAddress) {
			allErrs = append(allErrs, field.Invalid(fldPath.Child("urls"), strings.Join(clientURLs.List(), ","), fmt.Sprintf("must include the etcd address %s", builtInAddress)))
		}
	}

	return allErrs
}

func ValidateEtcdConfig(config *config.EtcdConfig, fldPath *field.Path) ValidationResults {
	validationResults := ValidationResults{}

	servingInfoPath := fldPath.Child("servingInfo")
	validationResults.Append(ValidateServingInfo(config.ServingInfo, true, servingInfoPath))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(field.Invalid(servingInfoPath.Child("bindNetwork"), config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for etcd, must be tcp or tcp4"))
	}
	if len(config.ServingInfo.NamedCertificates) > 0 {
		validationResults.AddErrors(field.Invalid(servingInfoPath.Child("namedCertificates"), "<not shown>", "namedCertificates are not supported for etcd"))
	}

	peerServingInfoPath := fldPath.Child("peerServingInfo")
	validationResults.Append(ValidateServingInfo(config.PeerServingInfo, true, peerServingInfoPath))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(field.Invalid(peerServingInfoPath.Child("bindNetwork"), config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for etcd peers, must be tcp or tcp4"))
	}
	if len(config.ServingInfo.NamedCertificates) > 0 {
		validationResults.AddErrors(field.Invalid(peerServingInfoPath.Child("namedCertificates"), "<not shown>", "namedCertificates are not supported for etcd"))
	}

	validationResults.AddErrors(ValidateHostPort(config.Address, fldPath.Child("address"))...)
	validationResults.AddErrors(ValidateHostPort(config.PeerAddress, fldPath.Child("peerAddress"))...)

	if len(config.StorageDir) == 0 {
		validationResults.AddErrors(field.Required(fldPath.Child("storageDirectory"), ""))
	}

	return validationResults
}
