package api

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
		&DeploymentConfig{},
		&DeploymentConfigList{},
		&DeploymentConfigRollback{},
	)
}

func (*DeploymentConfig) IsAnAPIObject()         {}
func (*DeploymentConfigList) IsAnAPIObject()     {}
func (*DeploymentConfigRollback) IsAnAPIObject() {}
