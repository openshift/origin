package openshift_sdn

import (
	"bytes"
	"io/ioutil"

	legacyconfigv1 "github.com/openshift/api/legacyconfig/v1"
	"github.com/openshift/library-go/pkg/config/helpers"
)

func readNodeConfig(filename string) (*legacyconfigv1.NodeConfig, error) {
	data, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	uncast, err := helpers.ReadYAML(bytes.NewBuffer(data), legacyconfigv1.InstallLegacy)
	if err != nil {
		return nil, err
	}

	ret := uncast.(*legacyconfigv1.NodeConfig)
	// at this point defaults need to be set
	setDefaults_NodeConfig(ret)

	return ret, nil
}

func setDefaults_NodeConfig(obj *legacyconfigv1.NodeConfig) {
	if len(obj.IPTablesSyncPeriod) == 0 {
		obj.IPTablesSyncPeriod = "30s"
	}

	// EnableUnidling by default
	if obj.EnableUnidling == nil {
		v := true
		obj.EnableUnidling = &v
	}
}
