package configconversion

import (
	"net"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	externaliprangerv1 "github.com/openshift/origin/pkg/service/admission/apis/externalipranger/v1"
	restrictedendpointsv1 "github.com/openshift/origin/pkg/service/admission/apis/restrictedendpoints/v1"
)

func convertNetworkConfigToAdmissionConfig(masterConfig *legacyconfigv1.MasterConfig) error {
	if masterConfig.AdmissionConfig.PluginConfig == nil {
		masterConfig.AdmissionConfig.PluginConfig = map[string]*legacyconfigv1.AdmissionPluginConfig{}
	}

	scheme := runtime.NewScheme()
	utilruntime.Must(externaliprangerv1.InstallLegacy(scheme))
	utilruntime.Must(restrictedendpointsv1.InstallLegacy(scheme))
	codecs := serializer.NewCodecFactory(scheme)
	encoder := codecs.LegacyCodec(externaliprangerv1.SchemeGroupVersion, restrictedendpointsv1.SchemeGroupVersion)

	// convert the networkconfig to admissionconfig
	var restricted []string
	restricted = append(restricted, masterConfig.NetworkConfig.ServiceNetworkCIDR)
	for _, cidr := range masterConfig.NetworkConfig.ClusterNetworks {
		restricted = append(restricted, cidr.CIDR)
	}
	restrictedEndpointConfig := &restrictedendpointsv1.RestrictedEndpointsAdmissionConfig{
		RestrictedCIDRs: restricted,
	}
	restrictedEndpointConfigContent, err := runtime.Encode(encoder, restrictedEndpointConfig)
	if err != nil {
		return err
	}
	masterConfig.AdmissionConfig.PluginConfig["openshift.io/RestrictedEndpointsAdmission"] = &legacyconfigv1.AdmissionPluginConfig{
		Configuration: runtime.RawExtension{Raw: restrictedEndpointConfigContent},
	}

	allowIngressIP := false
	if _, ipNet, err := net.ParseCIDR(masterConfig.NetworkConfig.IngressIPNetworkCIDR); err == nil && !ipNet.IP.IsUnspecified() {
		allowIngressIP = true
	}
	externalIPRangerAdmissionConfig := &externaliprangerv1.ExternalIPRangerAdmissionConfig{
		ExternalIPNetworkCIDRs: masterConfig.NetworkConfig.ExternalIPNetworkCIDRs,
		AllowIngressIP:         allowIngressIP,
	}
	externalIPRangerAdmissionConfigContent, err := runtime.Encode(encoder, externalIPRangerAdmissionConfig)
	if err != nil {
		return err
	}
	masterConfig.AdmissionConfig.PluginConfig["ExternalIPRanger"] = &legacyconfigv1.AdmissionPluginConfig{
		Configuration: runtime.RawExtension{Raw: externalIPRangerAdmissionConfigContent},
	}

	return nil
}

// ConvertMasterConfigToKubeAPIServerConfig mutates it's input.  This is acceptable because we do not need it by the time we get to 4.0.
func ConvertMasterConfigToKubeAPIServerConfig(input *legacyconfigv1.MasterConfig) (*kubecontrolplanev1.KubeAPIServerConfig, error) {
	if err := convertNetworkConfigToAdmissionConfig(input); err != nil {
		return nil, err
	}

	var err error

	ret := &kubecontrolplanev1.KubeAPIServerConfig{
		GenericAPIServerConfig: configv1.GenericAPIServerConfig{
			CORSAllowedOrigins: input.CORSAllowedOrigins,
			StorageConfig: configv1.EtcdStorageConfig{
				StoragePrefix: input.EtcdStorageConfig.OpenShiftStoragePrefix,
			},
		},

		ServicesSubnet:        input.KubernetesMasterConfig.ServicesSubnet,
		ServicesNodePortRange: input.KubernetesMasterConfig.ServicesNodePortRange,

		LegacyServiceServingCertSignerCABundle: input.ControllerConfig.ServiceServingCert.Signer.CertFile,

		ImagePolicyConfig: kubecontrolplanev1.KubeAPIServerImagePolicyConfig{
			InternalRegistryHostname: input.ImagePolicyConfig.InternalRegistryHostname,
			ExternalRegistryHostname: input.ImagePolicyConfig.ExternalRegistryHostname,
		},

		ProjectConfig: kubecontrolplanev1.KubeAPIServerProjectConfig{
			DefaultNodeSelector: input.ProjectConfig.DefaultNodeSelector,
		},

		ServiceAccountPublicKeyFiles: input.ServiceAccountConfig.PublicKeyFiles,

		// TODO this needs to be removed.
		APIServerArguments: map[string]kubecontrolplanev1.Arguments{},
	}
	for k, v := range input.KubernetesMasterConfig.APIServerArguments {
		ret.APIServerArguments[k] = v
	}

	// TODO this is likely to be a little weird.  I think we override most of this in the operator
	ret.ServingInfo, err = ToHTTPServingInfo(&input.ServingInfo)
	if err != nil {
		return nil, err
	}

	ret.OAuthConfig, err = ToOAuthConfig(input.OAuthConfig)
	if err != nil {
		return nil, err
	}

	ret.AuthConfig, err = ToMasterAuthConfig(&input.AuthConfig)
	if err != nil {
		return nil, err
	}

	ret.AggregatorConfig, err = ToAggregatorConfig(&input.AggregatorConfig)
	if err != nil {
		return nil, err
	}

	ret.AuditConfig, err = ToAuditConfig(&input.AuditConfig)
	if err != nil {
		return nil, err
	}

	ret.StorageConfig.EtcdConnectionInfo, err = ToEtcdConnectionInfo(&input.EtcdClientInfo)
	if err != nil {
		return nil, err
	}

	ret.KubeletClientInfo, err = ToKubeletConnectionInfo(&input.KubeletClientInfo)
	if err != nil {
		return nil, err
	}

	ret.AdmissionPluginConfig, err = ToAdmissionPluginConfigMap(input.AdmissionConfig.PluginConfig)
	if err != nil {
		return nil, err
	}

	ret.UserAgentMatchingConfig, err = ToUserAgentMatchingConfig(&input.PolicyConfig.UserAgentMatchingConfig)
	if err != nil {
		return nil, err
	}

	return ret, nil
}
