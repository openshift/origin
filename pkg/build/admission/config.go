package admission

import (
	"io"
	"io/ioutil"
	"reflect"

	"k8s.io/kubernetes/pkg/runtime"

	configlatest "github.com/openshift/origin/pkg/cmd/server/api/latest"
)

// ReadPluginConfig will read a plugin configuration object from a reader stream
func ReadPluginConfig(reader io.Reader, config runtime.Object) error {
	if reader == nil || reflect.ValueOf(reader).IsNil() {
		return nil
	}

	configBytes, err := ioutil.ReadAll(reader)
	if err != nil {
		return err
	}
	err = configlatest.ReadYAMLInto(configBytes, config)
	if err != nil {
		return err
	}
	return nil
}
