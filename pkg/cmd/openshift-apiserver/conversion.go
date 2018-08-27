package openshift_apiserver

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
)

func ConvertMasterConfigToOpenshiftAPIServerConfig(input *configapi.MasterConfig) *configapi.OpenshiftAPIServerConfig {
	ret := &configapi.OpenshiftAPIServerConfig{
		// TODO this is likely to be a little weird.  I think we override most of this in the operator
		ServingInfo:        input.ServingInfo,
		CORSAllowedOrigins: input.CORSAllowedOrigins,

		MasterClients: input.MasterClients,
		AuditConfig:   input.AuditConfig,

		StoragePrefix:  input.EtcdStorageConfig.OpenShiftStoragePrefix,
		EtcdClientInfo: input.EtcdClientInfo,

		ImagePolicyConfig: configapi.ServerImagePolicyConfig{
			MaxImagesBulkImportedPerRepository: input.ImagePolicyConfig.MaxImagesBulkImportedPerRepository,
			AllowedRegistriesForImport:         input.ImagePolicyConfig.AllowedRegistriesForImport,
			InternalRegistryHostname:           input.ImagePolicyConfig.InternalRegistryHostname,
			ExternalRegistryHostname:           input.ImagePolicyConfig.ExternalRegistryHostname,
			AdditionalTrustedCA:                input.ImagePolicyConfig.AdditionalTrustedCA,
		},

		ProjectConfig: configapi.ServerProjectConfig{
			DefaultNodeSelector:    input.ProjectConfig.DefaultNodeSelector,
			ProjectRequestMessage:  input.ProjectConfig.ProjectRequestMessage,
			ProjectRequestTemplate: input.ProjectConfig.ProjectRequestTemplate,
		},

		RoutingConfig: input.RoutingConfig,

		AdmissionPluginConfig: input.AdmissionConfig.PluginConfig,
		// TODO this is logically an admission configuration
		JenkinsPipelineConfig: input.JenkinsPipelineConfig,

		// TODO this needs to be removed.
		APIServerArguments: input.KubernetesMasterConfig.APIServerArguments,
	}
	if input.OAuthConfig != nil {
		ret.ServiceAccountOAuthGrantMethod = input.OAuthConfig.GrantConfig.ServiceAccountMethod
	}

	return ret
}
