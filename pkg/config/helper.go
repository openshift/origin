package config

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/meta"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/api/latest"
)

// Set the default RESTMapper and ObjectTyper
var (
	defaultMapper = latest.RESTMapper
	defaultTyper  = api.Scheme
)

// DecodeDataToObject decodes the JSON/YAML content into the runtime Object
// using the RESTMapper interface.
// The RESTMapper mappings are returned for the future encoding.
func DecodeDataToObject(data []byte) (obj runtime.Object, mapping *meta.RESTMapping, err error) {
	version, kind, err := defaultTyper.DataVersionAndKind(data)
	if err != nil {
		return
	}

	mapping, err = defaultMapper.RESTMapping(version, kind)
	if err != nil {
		return
	}

	obj, err = mapping.Codec.Decode(data)
	return
}
