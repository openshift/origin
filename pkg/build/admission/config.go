package admission

import (
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/runtime"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
	"github.com/openshift/origin/pkg/cmd/util/pluginconfig"
)

// ReadPluginConfig will read a plugin configuration object from a reader stream
func ReadPluginConfig(pluginConfig map[string]configapi.AdmissionPluginConfig, name string, config runtime.Object) error {

	configFilePath, err := pluginconfig.GetPluginConfigFile(pluginConfig, name, "")
	if configFilePath == "" {
		return nil
	}

	err = configlatest.ReadYAMLFileInto(configFilePath, config)
	if err != nil {
		glog.Errorf("couldn't open plugin configuration %s: %#v", configFilePath, err)
		return err
	}
	return nil
}
