package rollback

import (
	"fmt"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
)

// RollbackGenerator generates a new DeploymentConfig by merging a pair of DeploymentConfigs
// in a configurable way.
type RollbackGenerator struct{}

// Generate creates a new DeploymentConfig by merging to onto from based on the options provided
// by spec. The LatestVersion of the result is unconditionally incremented, as rollback candidates are
// should be possible to be deployed manually regardless of other system behavior such as triggering.
func (g *RollbackGenerator) GenerateRollback(from, to *deployapi.DeploymentConfig, spec *deployapi.DeploymentConfigRollbackSpec) (*deployapi.DeploymentConfig, error) {
	rollback := &deployapi.DeploymentConfig{}

	if err := kapi.Scheme.Convert(&from, &rollback); err != nil {
		return nil, fmt.Errorf("couldn't clone 'from' DeploymentConfig: %v", err)
	}

	// construct the candidate deploymentConfig based on the rollback spec
	if spec.IncludeTemplate {
		if err := kapi.Scheme.Convert(&to.Template.ControllerTemplate.Template, &rollback.Template.ControllerTemplate.Template); err != nil {
			return nil, fmt.Errorf("couldn't copy template to rollback:: %v", err)
		}
	}

	if spec.IncludeReplicationMeta {
		rollback.Template.ControllerTemplate.Replicas = to.Template.ControllerTemplate.Replicas
		rollback.Template.ControllerTemplate.Selector = map[string]string{}
		for k, v := range to.Template.ControllerTemplate.Selector {
			rollback.Template.ControllerTemplate.Selector[k] = v
		}
	}

	if spec.IncludeTriggers {
		if err := kapi.Scheme.Convert(&to.Triggers, &rollback.Triggers); err != nil {
			return nil, fmt.Errorf("couldn't copy triggers to rollback:: %v", err)
		}
	}

	if spec.IncludeStrategy {
		if err := kapi.Scheme.Convert(&to.Template.Strategy, &rollback.Template.Strategy); err != nil {
			return nil, fmt.Errorf("couldn't copy strategy to rollback:: %v", err)
		}
	}

	// TODO: add a new cause?
	rollback.LatestVersion++

	return rollback, nil
}
