package admission

import (
	"io/ioutil"
	"os"

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
	configData, err := os.Open(configFilePath)
	if err != nil {
		glog.Fatalf("Couldn't open plugin configuration %s: %#v", configFilePath, err)
		return err
	}

	defer configData.Close()

	configBytes, err := ioutil.ReadAll(configData)

	err = configlatest.ReadYAMLInto(configBytes, config)
	if err != nil {
		return err
	}
	return nil
}
