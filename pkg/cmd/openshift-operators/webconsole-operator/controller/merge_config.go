package controller

import (
	"reflect"

	webconsoleconfigv1 "github.com/openshift/api/webconsole/v1"
	"github.com/openshift/origin/pkg/cmd/openshift-operators/util/resourcemerge"
)

func ensureWebConsoleConfiguration(modified *bool, existing *webconsoleconfigv1.WebConsoleConfiguration, required webconsoleconfigv1.WebConsoleConfiguration) {
	ensureHTTPServingInfo(modified, &existing.ServingInfo, required.ServingInfo)
	ensureClusterInfo(modified, &existing.ClusterInfo, required.ClusterInfo)
	ensureFeaturesConfiguration(modified, &existing.Features, required.Features)
	ensureExtensionsConfiguration(modified, &existing.Extensions, required.Extensions)
}

func ensureClusterInfo(modified *bool, existing *webconsoleconfigv1.ClusterInfo, required webconsoleconfigv1.ClusterInfo) {
	// TODO here's a neat side-effect.  You need to have everything be nil-able to know the difference between missing and explicitly set to "".
	resourcemerge.SetStringIfSet(modified, &existing.ConsolePublicURL, required.ConsolePublicURL)
	resourcemerge.SetStringIfSet(modified, &existing.MasterPublicURL, required.MasterPublicURL)
	resourcemerge.SetStringIfSet(modified, &existing.LoggingPublicURL, required.LoggingPublicURL)
	resourcemerge.SetStringIfSet(modified, &existing.MetricsPublicURL, required.MetricsPublicURL)
	resourcemerge.SetStringIfSet(modified, &existing.LogoutPublicURL, required.LogoutPublicURL)
}

func ensureFeaturesConfiguration(modified *bool, existing *webconsoleconfigv1.FeaturesConfiguration, required webconsoleconfigv1.FeaturesConfiguration) {
	// TODO here's a neat side-effect.  You need to have everything be nil-able to know the difference between missing and explicitly set to zero.
	resourcemerge.SetInt64(modified, &existing.InactivityTimeoutMinutes, required.InactivityTimeoutMinutes)
	// TODO here's a neat side-effect.  You need to have everything be nil-able to know the difference between missing and explicitly set to false.
	resourcemerge.SetBool(modified, &existing.ClusterResourceOverridesEnabled, required.ClusterResourceOverridesEnabled)
}

func ensureExtensionsConfiguration(modified *bool, existing *webconsoleconfigv1.ExtensionsConfiguration, required webconsoleconfigv1.ExtensionsConfiguration) {
	resourcemerge.SetStringSliceIfSet(modified, &existing.ScriptURLs, required.ScriptURLs)
	resourcemerge.SetStringSliceIfSet(modified, &existing.StylesheetURLs, required.StylesheetURLs)
	// this is overwritten as a whole, not merged
	resourcemerge.SetMapStringStringIfSet(modified, &existing.Properties, required.Properties)
}

func ensureHTTPServingInfo(modified *bool, existing *webconsoleconfigv1.HTTPServingInfo, required webconsoleconfigv1.HTTPServingInfo) {
	ensureServingInfo(modified, &existing.ServingInfo, required.ServingInfo)
	resourcemerge.SetInt64(modified, &existing.MaxRequestsInFlight, required.MaxRequestsInFlight)
	resourcemerge.SetInt64(modified, &existing.RequestTimeoutSeconds, required.RequestTimeoutSeconds)
}

func ensureServingInfo(modified *bool, existing *webconsoleconfigv1.ServingInfo, required webconsoleconfigv1.ServingInfo) {
	ensureCertInfo(modified, &existing.CertInfo, required.CertInfo)

	resourcemerge.SetStringIfSet(modified, &existing.BindAddress, required.BindAddress)
	resourcemerge.SetStringIfSet(modified, &existing.BindNetwork, required.BindNetwork)
	resourcemerge.SetStringIfSet(modified, &existing.ClientCA, required.ClientCA)
	resourcemerge.SetStringIfSet(modified, &existing.MinTLSVersion, required.MinTLSVersion)
	resourcemerge.SetStringIfSet(modified, &existing.BindNetwork, required.BindNetwork)
	resourcemerge.SetStringSlice(modified, &existing.CipherSuites, required.CipherSuites)

	// named certs are an all or nothing
	if required.NamedCertificates != nil {
		if !reflect.DeepEqual(existing.NamedCertificates, required.NamedCertificates) {
			*modified = true
			existing.NamedCertificates = required.NamedCertificates
		}
	}
}

func ensureCertInfo(modified *bool, existing *webconsoleconfigv1.CertInfo, required webconsoleconfigv1.CertInfo) {
	// cert info is always overwritten as whole, but only if it is set
	if len(required.CertFile) == 0 && len(required.KeyFile) == 0 {
		return
	}

	if existing == nil {
		*existing = webconsoleconfigv1.CertInfo{}
	}
	if *existing != required {
		*modified = true
		*existing = required
	}
}
