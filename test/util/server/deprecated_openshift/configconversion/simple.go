package configconversion

import (
	"k8s.io/kubernetes/pkg/kubeapiserver/options"

	configv1 "github.com/openshift/api/config/v1"
	kubecontrolplanev1 "github.com/openshift/api/kubecontrolplane/v1"
	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	osinv1 "github.com/openshift/api/osin/v1"
	"github.com/openshift/library-go/pkg/crypto"
	"github.com/openshift/openshift-apiserver/pkg/cmd/openshift-apiserver/openshiftadmission"
	"k8s.io/kubernetes/openshift-kube-apiserver/kubeadmission"
)

func ToHTTPServingInfo(in *legacyconfigv1.HTTPServingInfo) (out configv1.HTTPServingInfo, err error) {
	err = Convert_legacyconfigv1_HTTPServingInfo_to_configv1_HTTPServingInfo(in, &out, nil)
	if err != nil {
		return configv1.HTTPServingInfo{}, err
	}
	if len(out.CipherSuites) == 0 {
		out.CipherSuites = crypto.CipherSuitesToNamesOrDie(crypto.DefaultCiphers())
	}
	return out, nil
}

func ToKubeClientConfig(in *legacyconfigv1.MasterClients) (out configv1.KubeClientConfig, err error) {
	err = Convert_legacyconfigv1_MasterClients_to_configv1_KubeClientConfig(in, &out, nil)
	if err != nil {
		return configv1.KubeClientConfig{}, err
	}
	return out, nil
}

func ToAuditConfig(in *legacyconfigv1.AuditConfig) (out configv1.AuditConfig, err error) {
	// FIXME: drop this once we drop openshift start, this prevents unnecessary conversions
	// of the embeded CreationTimestamp in audit object
	in.PolicyConfiguration.Object = nil
	err = Convert_legacyconfigv1_AuditConfig_to_configv1_AuditConfig(in, &out, nil)
	if err != nil {
		return configv1.AuditConfig{}, err
	}
	return out, nil
}

func ToEtcdConnectionInfo(in *legacyconfigv1.EtcdConnectionInfo) (out configv1.EtcdConnectionInfo, err error) {
	err = Convert_legacyconfigv1_EtcdConnectionInfo_to_configv1_EtcdConnectionInfo(in, &out, nil)
	if err != nil {
		return configv1.EtcdConnectionInfo{}, err
	}
	return out, nil
}

func ToAdmissionPluginConfig(in *legacyconfigv1.AdmissionPluginConfig) (*configv1.AdmissionPluginConfig, error) {
	out := &configv1.AdmissionPluginConfig{}
	if err := Convert_legacyconfigv1_AdmissionPluginConfig_to_configv1_AdmissionPluginConfig(in, out, nil); err != nil {
		return nil, err
	}
	return out, nil
}

func ToOpenShiftAdmissionPluginConfigMap(in map[string]*legacyconfigv1.AdmissionPluginConfig) (out map[string]configv1.AdmissionPluginConfig, err error) {
	if in == nil {
		return nil, nil
	}

	out = map[string]configv1.AdmissionPluginConfig{}
	for k, v := range in {
		if !isKnownOpenShiftAdmissionPlugin(k) {
			continue
		}
		outV, err := ToAdmissionPluginConfig(v)
		if err != nil {
			return nil, err
		}
		out[k] = *outV

	}

	return out, nil
}

func ToOpenShiftAdmissionPluginList(in []string) (out []string) {
	if in == nil {
		return nil
	}

	for _, name := range in {
		if !isKnownOpenShiftAdmissionPlugin(name) {
			continue
		}
		out = append(out, name)
	}

	return out
}

func ToKubeAdmissionPluginList(in []string) (out []string) {
	if in == nil {
		return nil
	}

	for _, name := range in {
		if !isKnownKubeAdmissionPlugin(name) {
			continue
		}
		out = append(out, name)
	}

	return out
}

func isKnownOpenShiftAdmissionPlugin(pluginName string) bool {
	for _, plugin := range openshiftadmission.OpenShiftAdmissionPlugins {
		if pluginName == plugin {
			return true
		}
	}

	return false
}
func ToKubeAdmissionPluginConfigMap(in map[string]*legacyconfigv1.AdmissionPluginConfig) (out map[string]configv1.AdmissionPluginConfig, err error) {
	if in == nil {
		return nil, nil
	}

	out = map[string]configv1.AdmissionPluginConfig{}
	for k, v := range in {
		if !isKnownKubeAdmissionPlugin(k) {
			continue
		}
		outV, err := ToAdmissionPluginConfig(v)
		if err != nil {
			return nil, err
		}
		out[k] = *outV

	}

	return out, nil
}

func isKnownKubeAdmissionPlugin(pluginName string) bool {
	for _, plugin := range kubeadmission.NewOrderedKubeAdmissionPlugins(options.AllOrderedPlugins) {
		if pluginName == plugin {
			return true
		}
	}

	return false
}

func ToMasterAuthConfig(in *legacyconfigv1.MasterAuthConfig) (out kubecontrolplanev1.MasterAuthConfig, err error) {
	err = Convert_legacyconfigv1_MasterAuthConfig_to_kubecontrolplanev1_MasterAuthConfig(in, &out, nil)
	if err != nil {
		return kubecontrolplanev1.MasterAuthConfig{}, err
	}
	return out, nil
}

func ToAggregatorConfig(in *legacyconfigv1.AggregatorConfig) (out kubecontrolplanev1.AggregatorConfig, err error) {
	err = Convert_legacyconfigv1_AggregatorConfig_to_kubecontrolplanev1_AggregatorConfig(in, &out, nil)
	if err != nil {
		return kubecontrolplanev1.AggregatorConfig{}, err
	}
	return out, nil
}

func ToKubeletConnectionInfo(in *legacyconfigv1.KubeletConnectionInfo) (out kubecontrolplanev1.KubeletConnectionInfo, err error) {
	err = Convert_legacyconfigv1_KubeletConnectionInfo_to_kubecontrolplanev1_KubeletConnectionInfo(in, &out, nil)
	if err != nil {
		return kubecontrolplanev1.KubeletConnectionInfo{}, err
	}
	return out, nil
}

func ToUserAgentMatchingConfig(in *legacyconfigv1.UserAgentMatchingConfig) (out kubecontrolplanev1.UserAgentMatchingConfig, err error) {
	err = Convert_legacyconfigv1_UserAgentMatchingConfig_to_kubecontrolplanev1_UserAgentMatchingConfig(in, &out, nil)
	if err != nil {
		return kubecontrolplanev1.UserAgentMatchingConfig{}, err
	}
	return out, nil
}

func ToOAuthConfig(in *legacyconfigv1.OAuthConfig) (*osinv1.OAuthConfig, error) {
	if in == nil {
		return nil, nil
	}
	out := &osinv1.OAuthConfig{}
	if err := Convert_legacyconfigv1_OAuthConfig_to_osinv1_OAuthConfig(in, out, nil); err != nil {
		return nil, err
	}
	return out, nil
}

func ToJenkinsPipelineConfig(in *legacyconfigv1.JenkinsPipelineConfig) (out openshiftcontrolplanev1.JenkinsPipelineConfig, err error) {
	err = Convert_legacyconfigv1_JenkinsPipelineConfig_to_kubecontrolplanev1_JenkinsPipelineConfig(in, &out, nil)
	if err != nil {
		return openshiftcontrolplanev1.JenkinsPipelineConfig{}, err
	}
	return out, nil
}

func ToAllowedRegistries(in *legacyconfigv1.AllowedRegistries) (openshiftcontrolplanev1.AllowedRegistries, error) {
	if in == nil {
		return openshiftcontrolplanev1.AllowedRegistries{}, nil
	}

	out := openshiftcontrolplanev1.AllowedRegistries{}
	for i := range *in {
		currOut := openshiftcontrolplanev1.RegistryLocation{}
		err := Convert_legacyconfigv1_RegistryLocation_to_kubecontrolplanev1_RegistryLocation(&(*in)[i], &currOut, nil)
		if err != nil {
			return nil, err
		}
		out = append(out, currOut)
	}
	return out, nil
}
