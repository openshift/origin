package v1

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1",
		&DeploymentConfig{},
		&DeploymentConfigList{},
		&DeploymentConfigRollback{},
		&DeploymentLog{},
		&DeploymentLogOptions{},
	)
}

func (*DeploymentConfig) IsAnAPIObject()         {}
func (*DeploymentConfigList) IsAnAPIObject()     {}
func (*DeploymentConfigRollback) IsAnAPIObject() {}
func (*DeploymentLog) IsAnAPIObject()            {}
func (*DeploymentLogOptions) IsAnAPIObject()     {}
