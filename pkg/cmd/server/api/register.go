package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

var Scheme = runtime.NewScheme()

func init() {
	Scheme.AddKnownTypes("",
		&MasterConfig{},
		&NodeConfig{},
	)
}

func (*MasterConfig) IsAnAPIObject() {}
func (*NodeConfig) IsAnAPIObject()   {}
