package v1beta1

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta1",
		&Deployment{},
		&DeploymentList{},
		&DeploymentConfig{},
		&DeploymentConfigList{},
	)
}

func (*Deployment) IsAnAPIObject()           {}
func (*DeploymentList) IsAnAPIObject()       {}
func (*DeploymentConfig) IsAnAPIObject()     {}
func (*DeploymentConfigList) IsAnAPIObject() {}
