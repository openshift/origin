package pluginconfig

import (
	"io/ioutil"

	configapi "github.com/openshift/origin/pkg/cmd/server/api"
	"github.com/openshift/origin/pkg/cmd/server/api/latest"
)

func GetPluginConfig(cfg configapi.AdmissionPluginConfig) (string, error) {
	obj := cfg.Configuration
	if obj == nil {
		return cfg.Location, nil
	}

	configFile, err := ioutil.TempFile("", "admission-plugin-config")
	if err != nil {
		return "", err
	}
	if err = configFile.Close(); err != nil {
		return "", err
	}
	content, err := latest.WriteYAML(obj)
	if err != nil {
		return "", err
	}
	if err = ioutil.WriteFile(configFile.Name(), content, 0644); err != nil {
		return "", err
	}
	return configFile.Name(), nil
}
