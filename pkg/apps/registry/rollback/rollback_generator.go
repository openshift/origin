package rollback

import (
	"fmt"

	"k8s.io/kubernetes/pkg/api/legacyscheme"

	appsv1 "github.com/openshift/api/apps/v1"
	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
)

// RollbackGenerator generates a new deployment config by merging a pair of deployment
// configs in a configurable way.
type RollbackGenerator interface {
	// GenerateRollback creates a new deployment config by merging to onto from
	// based on the options provided by spec. The latestVersion of the result is
	// unconditionally incremented, as rollback candidates should be possible
	// to be deployed manually regardless of other system behavior such as
	// triggering.
	//
	// Any image change triggers on the new config are disabled to prevent
	// triggered deployments from immediately replacing the rollback.
	GenerateRollback(from, to *appsv1.DeploymentConfig, spec *appsv1.DeploymentConfigRollbackSpec) (*appsapi.DeploymentConfig, error)
}

// NewRollbackGenerator returns a new rollback generator.
func NewRollbackGenerator() RollbackGenerator {
	return &rollbackGenerator{}
}

type rollbackGenerator struct{}

func (g *rollbackGenerator) GenerateRollback(from, to *appsv1.DeploymentConfig, spec *appsv1.DeploymentConfigRollbackSpec) (*appsapi.DeploymentConfig, error) {
	rollback := &appsv1.DeploymentConfig{}

	if err := legacyscheme.Scheme.Convert(&from, &rollback, nil); err != nil {
		return nil, fmt.Errorf("couldn't clone 'from' DeploymentConfig: %v", err)
	}

	// construct the candidate deploymentConfig based on the rollback spec
	if spec.IncludeTemplate {
		rollback.Spec.Template = to.Spec.Template.DeepCopy()
	}

	if spec.IncludeReplicationMeta {
		rollback.Spec.Replicas = to.Spec.Replicas
		rollback.Spec.Selector = map[string]string{}
		for k, v := range to.Spec.Selector {
			rollback.Spec.Selector[k] = v
		}
	}

	if spec.IncludeTriggers {
		rollback.Spec.Triggers = to.Spec.Triggers.DeepCopy()
	}

	if spec.IncludeStrategy {
		rollback.Spec.Strategy = to.Spec.Strategy
	}

	// Disable any image change triggers.
	for _, trigger := range rollback.Spec.Triggers {
		if trigger.Type == appsv1.DeploymentTriggerOnImageChange {
			trigger.ImageChangeParams.Automatic = false
		}
	}

	// TODO: add a new cause?
	// TODO: Instantiate instead of incrementing latestVersion
	rollback.Status.LatestVersion++

	rollbackInternal := &appsapi.DeploymentConfig{}
	if err := legacyscheme.Scheme.Convert(rollback, rollbackInternal, nil); err != nil {
		return nil, err
	}

	return rollbackInternal, nil
}
