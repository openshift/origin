package api

import (
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/registry/generic"
)

// DeploymentConfigToSelectableFields returns a label set that represents the object
func DeploymentConfigToSelectableFields(deploymentConfig *DeploymentConfig) fields.Set {
	return generic.ObjectMetaFieldsSet(&deploymentConfig.ObjectMeta, true)
}
