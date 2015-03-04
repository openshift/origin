package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	Scheme.AddKnownTypes("",
		&OpenShiftMasterConfig{},
		&KubernetesMasterConfig{},
		&NodeConfig{},
	)
}

func (*OpenShiftMasterConfig) IsAnAPIObject()  {}
func (*KubernetesMasterConfig) IsAnAPIObject() {}
func (*NodeConfig) IsAnAPIObject()             {}
