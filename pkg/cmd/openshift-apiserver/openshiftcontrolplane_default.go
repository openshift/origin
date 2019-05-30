package openshift_apiserver

import (
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/configdefaults"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func setRecommendedOpenShiftAPIServerConfigDefaults(config *openshiftcontrolplanev1.OpenShiftAPIServerConfig) {
	configdefaults.DefaultString(&config.GenericAPIServerConfig.StorageConfig.StoragePrefix, "openshift.io")

	configdefaults.SetRecommendedGenericAPIServerConfigDefaults(&config.GenericAPIServerConfig)

	configdefaults.DefaultString(&config.RoutingConfig.Subdomain, "router.default.svc.cluster.local")
	configdefaults.DefaultString(&config.JenkinsPipelineConfig.TemplateNamespace, "openshift")
	configdefaults.DefaultString(&config.JenkinsPipelineConfig.TemplateName, "jenkins-ephemeral")
	configdefaults.DefaultString(&config.JenkinsPipelineConfig.ServiceName, "jenkins")
	if len(config.ServiceAccountOAuthGrantMethod) == 0 {
		config.ServiceAccountOAuthGrantMethod = openshiftcontrolplanev1.GrantHandlerPrompt
	}

	if config.ImagePolicyConfig.MaxImagesBulkImportedPerRepository == 0 {
		config.ImagePolicyConfig.MaxImagesBulkImportedPerRepository = 50
	}
}

func getOpenShiftAPIServerConfigFileReferences(config *openshiftcontrolplanev1.OpenShiftAPIServerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	refs = append(refs, helpers.GetGenericAPIServerConfigFileReferences(&config.GenericAPIServerConfig)...)
	refs = append(refs, &config.ImagePolicyConfig.AdditionalTrustedCA)

	return refs
}
