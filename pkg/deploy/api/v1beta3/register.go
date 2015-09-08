package v1beta3

import (
	"k8s.io/kubernetes/pkg/api"
)

func init() {
	api.Scheme.AddKnownTypes("v1beta3",
		&DeploymentConfig{},
		&DeploymentConfigList{},
		&DeploymentConfigRollback{},
	)
}

func (*DeploymentConfig) IsAnAPIObject()         {}
func (*DeploymentConfigList) IsAnAPIObject()     {}
func (*DeploymentConfigRollback) IsAnAPIObject() {}
