package test

import (
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

// MakeTestOnlyInternalDeployment makes a test deployment (replication controller) from given deployment config.
// DEPRECATED: This is only used in some registry tests because storage is using internal versions for now.
func MakeTestOnlyInternalDeployment(config *appsapi.DeploymentConfig) (*kapi.ReplicationController, error) {
	configExternal := &appsv1.DeploymentConfig{}
	if err := legacyscheme.Scheme.Convert(config, configExternal, nil); err != nil {
		return nil, err
	}
	deploymentExternal, err := appsutil.MakeDeployment(configExternal)
	if err != nil {
		return nil, err
	}
	deploymentInternal := &kapi.ReplicationController{}
	if err := legacyscheme.Scheme.Convert(deploymentExternal, deploymentInternal, nil); err != nil {
		return nil, err
	}
	return deploymentInternal, nil
}
