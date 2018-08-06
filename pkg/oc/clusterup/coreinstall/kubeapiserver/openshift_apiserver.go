package kubeapiserver

import (
	"path"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/runtime"

	configapi "github.com/openshift/origin/pkg/cmd/server/apis/config/v1"
	"github.com/openshift/origin/pkg/oc/clusteradd/componentinstall"
	"github.com/openshift/origin/pkg/oc/clusterup/coreinstall/tmpformac"
)

func MakeOpenShiftAPIServerConfig(existingMasterConfig string, routingSuffix, basedir string) (string, error) {
	configDir := path.Join(basedir, OpenShiftAPIServerDirName)
	glog.V(1).Infof("Copying kube-apiserver config to local directory %s", configDir)
	err := tmpformac.CopyDirectory(existingMasterConfig, configDir)
	if err != nil {
		return "", err
	}

	// update some listen information to include starting the DNS server
	masterconfigFilename := path.Join(configDir, "master-config.yaml")
	masterconfig, err := componentinstall.ReadMasterConfig(masterconfigFilename)
	if err != nil {
		return "", err
	}

	masterconfig.ServingInfo.BindAddress = "0.0.0.0:8445"

	// hardcode the route suffix to the old default.  If anyone wants to change it, they can modify their config.
	masterconfig.RoutingConfig.Subdomain = routingSuffix

	// use the generated service serving cert
	masterconfig.ServingInfo.CertFile = "/var/serving-cert/tls.crt"
	masterconfig.ServingInfo.KeyFile = "/var/serving-cert/tls.key"

	// default openshift image policy admission
	if masterconfig.AdmissionConfig.PluginConfig == nil {
		masterconfig.AdmissionConfig.PluginConfig = map[string]*configapi.AdmissionPluginConfig{}
	}

	// Add default ImagePolicyConfig into openshift api master config
	policyConfig := []byte(`{"kind":"ImagePolicyConfig","apiVersion":"v1","executionRules":[{"name":"execution-denied",
"onResources":[{"resource":"pods"},{"resource":"builds"}],"reject":true,"matchImageAnnotations":[{"key":"images.openshift.io/deny-execution",
"value":"true"}],"skipOnResolutionFailure":true}]}`)

	masterconfig.AdmissionConfig.PluginConfig["openshift.io/ImagePolicy"] = &configapi.AdmissionPluginConfig{
		Configuration: runtime.RawExtension{Raw: policyConfig},
	}

	if err := componentinstall.WriteMasterConfig(masterconfigFilename, masterconfig); err != nil {
		return "", err
	}

	return configDir, nil
}
