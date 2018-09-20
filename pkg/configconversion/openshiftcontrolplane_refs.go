package configconversion

import (
	openshiftcontrolplanev1 "github.com/openshift/api/openshiftcontrolplane/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func GetOpenShiftAPIServerConfigFileReferences(config *openshiftcontrolplanev1.OpenShiftAPIServerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	refs = append(refs, helpers.GetGenericAPIServerConfigFileReferences(&config.GenericAPIServerConfig)...)
	refs = append(refs, &config.ImagePolicyConfig.AdditionalTrustedCA)

	return refs
}

func GetOpenShiftControllerConfigFileReferences(config *openshiftcontrolplanev1.OpenShiftControllerManagerConfig) []*string {
	if config == nil {
		return []*string{}
	}

	refs := []*string{}

	refs = append(refs, helpers.GetHTTPServingInfoFileReferences(config.ServingInfo)...)
	refs = append(refs, helpers.GetKubeClientConfigFileReferences(&config.KubeClientConfig)...)

	return refs
}
