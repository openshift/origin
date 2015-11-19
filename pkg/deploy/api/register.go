package api

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("",
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
