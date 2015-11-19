package validation

import (
	"fmt"
	"strings"

	"github.com/openshift/origin/pkg/cmd/server/api"
	"k8s.io/kubernetes/pkg/util/fielderrors"
	"k8s.io/kubernetes/pkg/util/sets"
)

// ValidateEtcdConnectionInfo validates the connection info. If a server EtcdConfig is provided,
// it ensures the connection info includes a URL for it, and has a client cert/key if the server requires
// client certificate authentication
func ValidateEtcdConnectionInfo(config api.EtcdConnectionInfo, server *api.EtcdConfig) fielderrors.ValidationErrorList {
	allErrs := fielderrors.ValidationErrorList{}

	if len(config.URLs) == 0 {
		allErrs = append(allErrs, fielderrors.NewFieldRequired("urls"))
	}
	for i, u := range config.URLs {
		_, urlErrs := ValidateURL(u, fmt.Sprintf("urls[%d]", i))
		if len(urlErrs) > 0 {
			allErrs = append(allErrs, urlErrs...)
		}
	}

	if len(config.CA) > 0 {
		allErrs = append(allErrs, ValidateFile(config.CA, "ca")...)
	}
	allErrs = append(allErrs, ValidateCertInfo(config.ClientCert, false)...)

	// If we have server config info, make sure the client connection info will work with it
	if server != nil {
		var builtInAddress string
		if api.UseTLS(server.ServingInfo) {
			builtInAddress = fmt.Sprintf("https://%s", server.Address)
		} else {
			builtInAddress = fmt.Sprintf("http://%s", server.Address)
		}

		// Require a client cert to connect to an etcd that requires client certs
		if len(server.ServingInfo.ClientCA) > 0 {
			if len(config.ClientCert.CertFile) == 0 {
				allErrs = append(allErrs, fielderrors.NewFieldRequired("certFile"))
			}
		}

		// Require the etcdClientInfo to include the address of the internal etcd
		clientURLs := sets.NewString(config.URLs...)
		if !clientURLs.Has(builtInAddress) {
			allErrs = append(allErrs, fielderrors.NewFieldInvalid("urls", strings.Join(clientURLs.List(), ","), fmt.Sprintf("must include the etcd address %s", builtInAddress)))
		}
	}

	return allErrs
}

func ValidateEtcdConfig(config *api.EtcdConfig) ValidationResults {
	validationResults := ValidationResults{}

	validationResults.Append(ValidateServingInfo(config.ServingInfo).Prefix("servingInfo"))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("servingInfo.bindNetwork", config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for etcd, must be tcp or tcp4"))
	}
	if len(config.ServingInfo.NamedCertificates) > 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("servingInfo.namedCertificates", "<not shown>", "namedCertificates are not supported for etcd"))
	}

	validationResults.Append(ValidateServingInfo(config.PeerServingInfo).Prefix("peerServingInfo"))
	if config.ServingInfo.BindNetwork == "tcp6" {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("peerServingInfo.bindNetwork", config.ServingInfo.BindNetwork, "tcp6 is not a valid bindNetwork for etcd peers, must be tcp or tcp4"))
	}
	if len(config.ServingInfo.NamedCertificates) > 0 {
		validationResults.AddErrors(fielderrors.NewFieldInvalid("peerServingInfo.namedCertificates", "<not shown>", "namedCertificates are not supported for etcd"))
	}

	validationResults.AddErrors(ValidateHostPort(config.Address, "address")...)
	validationResults.AddErrors(ValidateHostPort(config.PeerAddress, "peerAddress")...)

	if len(config.StorageDir) == 0 {
		validationResults.AddErrors(fielderrors.NewFieldRequired("storageDirectory"))
	}

	return validationResults
}
