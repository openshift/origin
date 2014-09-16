package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
)

func init() {
	runtime.AddKnownTypes("",
		DeploymentList{},
		Deployment{},
		DeploymentConfigList{},
		DeploymentConfig{},
	)
}
