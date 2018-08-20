package openshift_kube_apiserver

import (
	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config"
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

	return nil
}
