package rollback

import (
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
)

func TestGeneration(t *testing.T) {
	from := deploytest.OkDeploymentConfig(2)
	from.Template.Strategy = deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeCustom,
	}
	from.Triggers = append(from.Triggers, deployapi.DeploymentTriggerPolicy{Type: deployapi.DeploymentTriggerOnConfigChange})
	from.Triggers = append(from.Triggers, deploytest.OkImageChangeTrigger())
	from.Template.ControllerTemplate.Template.Spec.Containers[0].Name = "changed"
	from.Template.ControllerTemplate.Replicas = 5
	from.Template.ControllerTemplate.Selector = map[string]string{
		"new1": "new2",
		"new2": "new2",
	}

	to := deploytest.OkDeploymentConfig(1)

	// Generate a rollback for every combination of flag (using 1 bit per flag).
	rollbackSpecs := []*deployapi.DeploymentConfigRollbackSpec{}
	for i := 0; i < 15; i++ {
		spec := &deployapi.DeploymentConfigRollbackSpec{
			From: kapi.ObjectReference{
				Name:      "deployment",
				Namespace: kapi.NamespaceDefault,
			},
			IncludeTriggers:        i&(1<<0) > 0,
			IncludeTemplate:        i&(1<<1) > 0,
			IncludeReplicationMeta: i&(1<<2) > 0,
			IncludeStrategy:        i&(1<<3) > 0,
		}
		rollbackSpecs = append(rollbackSpecs, spec)
	}

	generator := &RollbackGenerator{}

	// Test every combination.
	for _, spec := range rollbackSpecs {
		t.Logf("testing spec %#v", spec)

		if rollback, err := generator.GenerateRollback(from, to, spec); err != nil {
			t.Fatalf("Unexpected error: %v", err)
		} else {
			if hasStrategyDiff(from, rollback) && !spec.IncludeStrategy {
				t.Fatalf("unexpected strategy diff: from=%v, rollback=%v", from, rollback)
			}

			if hasTriggerDiff(from, rollback) && !spec.IncludeTriggers {
				t.Fatalf("unexpected trigger diff: from=%v, rollback=%v", from, rollback)
			}

			if hasPodTemplateDiff(from, rollback) && !spec.IncludeTemplate {
				t.Fatalf("unexpected template diff: from=%v, rollback=%v", from, rollback)
			}

			if hasReplicationMetaDiff(from, rollback) && !spec.IncludeReplicationMeta {
				t.Fatalf("unexpected replication meta diff: from=%v, rollback=%v", from, rollback)
			}

			for i, trigger := range rollback.Triggers {
				if trigger.Type == deployapi.DeploymentTriggerOnImageChange && trigger.ImageChangeParams.Automatic {
					t.Errorf("image change trigger %d should be disabled", i)
				}
			}
		}
	}
}

func hasStrategyDiff(a, b *deployapi.DeploymentConfig) bool {
	return a.Template.Strategy.Type != b.Template.Strategy.Type
}

func hasTriggerDiff(a, b *deployapi.DeploymentConfig) bool {
	if len(a.Triggers) != len(b.Triggers) {
		return true
	}

	for _, triggerA := range a.Triggers {
		bHasTrigger := false
		for _, triggerB := range b.Triggers {
			if triggerB.Type == triggerA.Type {
				bHasTrigger = true
				break
			}
		}

		if !bHasTrigger {
			return true
		}
	}

	return false
}

func hasReplicationMetaDiff(a, b *deployapi.DeploymentConfig) bool {
	if a.Template.ControllerTemplate.Replicas != b.Template.ControllerTemplate.Replicas {
		return true
	}

	for keyA, valueA := range a.Template.ControllerTemplate.Selector {
		if valueB, exists := b.Template.ControllerTemplate.Selector[keyA]; !exists || valueA != valueB {
			return true
		}
	}

	return false
}

func hasPodTemplateDiff(a, b *deployapi.DeploymentConfig) bool {
	specA, specB := a.Template.ControllerTemplate.Template.Spec, b.Template.ControllerTemplate.Template.Spec
	return !kapi.Semantic.DeepEqual(specA, specB)
}
