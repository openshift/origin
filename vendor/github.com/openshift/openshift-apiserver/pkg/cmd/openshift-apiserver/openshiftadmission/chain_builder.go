package openshiftadmission

import (
	"io/ioutil"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/sets"
	apiserverv1alpha1 "k8s.io/apiserver/pkg/apis/apiserver/v1alpha1"

	configv1 "github.com/openshift/api/config/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func convertOpenshiftAdmissionConfigToKubeAdmissionConfig(in map[string]configv1.AdmissionPluginConfig) (*apiserverv1alpha1.AdmissionConfiguration, error) {
	ret := &apiserverv1alpha1.AdmissionConfiguration{}

	for _, pluginName := range sets.StringKeySet(in).List() {
		openshiftConfig := in[pluginName]

		kubeConfig := apiserverv1alpha1.AdmissionPluginConfiguration{
			Name: pluginName,
			Path: openshiftConfig.Location,
		}

		kubeConfig.Configuration = &runtime.Unknown{
			Raw: openshiftConfig.Configuration.Raw,
		}
		ret.Plugins = append(ret.Plugins, kubeConfig)
	}

	return ret, nil
}

func ToAdmissionConfigFile(pluginConfig map[string]configv1.AdmissionPluginConfig) (string, func(), error) {
	cleanupFn := func() {}
	upstreamAdmissionConfig, err := convertOpenshiftAdmissionConfigToKubeAdmissionConfig(pluginConfig)
	if err != nil {
		return "", cleanupFn, err
	}
	configBytes, err := helpers.WriteYAML(upstreamAdmissionConfig, apiserverv1alpha1.AddToScheme)
	if err != nil {
		return "", cleanupFn, err
	}

	tempFile, err := ioutil.TempFile("", "master-config.yaml")
	if err != nil {
		return "", cleanupFn, err
	}
	cleanupFn = func() { os.Remove(tempFile.Name()) }
	if _, err := tempFile.Write(configBytes); err != nil {
		return "", cleanupFn, err
	}
	tempFile.Close()

	return tempFile.Name(), cleanupFn, err

}
