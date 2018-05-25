package helpers

import (
	configv1 "github.com/openshift/api/config/v1"
)

func GetHTTPServingInfoFileReferences(config *configv1.HTTPServingInfo) []*string {
	return GetServingInfoFileReferences(&config.ServingInfo)
}

func GetServingInfoFileReferences(config *configv1.ServingInfo) []*string {
	refs := []*string{}

	refs = append(refs, GetCertFileReferences(&config.CertInfo)...)
	refs = append(refs, &config.ClientCA)
	for i := range config.NamedCertificates {
		refs = append(refs, &config.NamedCertificates[i].CertFile)
		refs = append(refs, &config.NamedCertificates[i].KeyFile)
	}

	return refs
}

func GetCertFileReferences(config *configv1.CertInfo) []*string {
	refs := []*string{}
	refs = append(refs, &config.CertFile)
	refs = append(refs, &config.KeyFile)
	return refs
}
