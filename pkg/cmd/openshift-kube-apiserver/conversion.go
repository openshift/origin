package openshift_kube_apiserver

import (
	"net"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
	"github.com/openshift/origin/pkg/service/admission/apis/externalipranger"
	"github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints"
)

func ConvertNetworkConfigToAdmissionConfig(masterConfig *configapi.MasterConfig) error {
	if masterConfig.AdmissionConfig.PluginConfig == nil {
		masterConfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{}
	}

	// convert the networkconfig to admissionconfig
	var restricted []string
	restricted = append(restricted, masterConfig.NetworkConfig.ServiceNetworkCIDR)
	for _, cidr := range masterConfig.NetworkConfig.ClusterNetworks {
		restricted = append(restricted, cidr.CIDR)
	}
	restrictedEndpointConfig := &restrictedendpoints.RestrictedEndpointsAdmissionConfig{
		RestrictedCIDRs: restricted,
	}
	masterConfig.AdmissionConfig.PluginConfig["openshift.io/RestrictedEndpointsAdmission"] = &configapi.AdmissionPluginConfig{
		Configuration: restrictedEndpointConfig,
	}

	allowIngressIP := false
	if _, ipNet, err := net.ParseCIDR(masterConfig.NetworkConfig.IngressIPNetworkCIDR); err == nil && !ipNet.IP.IsUnspecified() {
		allowIngressIP = true
	}
	externalIPRangerAdmissionConfig := &externalipranger.ExternalIPRangerAdmissionConfig{
		ExternalIPNetworkCIDRs: masterConfig.NetworkConfig.ExternalIPNetworkCIDRs,
		AllowIngressIP:         allowIngressIP,
	}
	masterConfig.AdmissionConfig.PluginConfig["ExternalIPRanger"] = &configapi.AdmissionPluginConfig{
		Configuration: externalIPRangerAdmissionConfig,
	}

	return nil
}

func ConvertMasterConfigToKubeAPIServerConfig(input *configapi.MasterConfig) *configapi.KubeAPIServerConfig {
	ret := &configapi.KubeAPIServerConfig{
		// TODO this is likely to be a little weird.  I think we override most of this in the operator
		ServingInfo:        input.ServingInfo,
		CORSAllowedOrigins: input.CORSAllowedOrigins,

		OAuthConfig:      input.OAuthConfig,
		AuthConfig:       input.AuthConfig,
		AggregatorConfig: input.AggregatorConfig,
		AuditConfig:      input.AuditConfig,

		StoragePrefix:  input.EtcdStorageConfig.OpenShiftStoragePrefix,
		EtcdClientInfo: input.EtcdClientInfo,

		KubeletClientInfo: input.KubeletClientInfo,

		AdmissionPluginConfig: input.AdmissionConfig.PluginConfig,

		ServicesSubnet:        input.KubernetesMasterConfig.ServicesSubnet,
		ServicesNodePortRange: input.KubernetesMasterConfig.ServicesNodePortRange,

		LegacyServiceServingCertSignerCABundle: input.ControllerConfig.ServiceServingCert.Signer.CertFile,

		UserAgentMatchingConfig: input.PolicyConfig.UserAgentMatchingConfig,

		ImagePolicyConfig: configapi.KubeAPIServerImagePolicyConfig{
			InternalRegistryHostname: input.ImagePolicyConfig.InternalRegistryHostname,
			ExternalRegistryHostname: input.ImagePolicyConfig.ExternalRegistryHostname,
		},

		ProjectConfig: configapi.KubeAPIServerProjectConfig{
			DefaultNodeSelector: input.ProjectConfig.DefaultNodeSelector,
		},

		ServiceAccountPublicKeyFiles: input.ServiceAccountConfig.PublicKeyFiles,

		// TODO this needs to be removed.
		APIServerArguments: input.KubernetesMasterConfig.APIServerArguments,
	}

	return ret
}
