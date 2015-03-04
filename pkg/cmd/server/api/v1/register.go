package v1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
	"github.com/openshift/origin/pkg/cmd/server/api"
)

var Codec = runtime.CodecFor(api.Scheme, "v1")

func init() {
	api.Scheme.AddKnownTypes("v1",
		&OpenShiftMasterConfig{},
		&KubernetesMasterConfig{},
		&NodeConfig{},
	)
}

func (*OpenShiftMasterConfig) IsAnAPIObject()  {}
func (*KubernetesMasterConfig) IsAnAPIObject() {}
func (*NodeConfig) IsAnAPIObject()             {}
