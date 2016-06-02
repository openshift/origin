package rollback

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
)

func TestGeneration(t *testing.T) {
	from := deploytest.OkDeploymentConfig(2)
	from.Spec.Strategy = deployapi.DeploymentStrategy{
		Type: deployapi.DeploymentStrategyTypeCustom,
	}
	from.Spec.Triggers = append(from.Spec.Triggers, deployapi.DeploymentTriggerPolicy{Type: deployapi.DeploymentTriggerOnConfigChange})
	from.Spec.Triggers = append(from.Spec.Triggers, deploytest.OkImageChangeTrigger())
	from.Spec.Template.Spec.Containers[0].Name = "changed"
	from.Spec.Replicas = 5
	from.Spec.Selector = map[string]string{
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

	generator := NewRollbackGenerator()

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

			for i, trigger := range rollback.Spec.Triggers {
				if trigger.Type == deployapi.DeploymentTriggerOnImageChange && trigger.ImageChangeParams.Automatic {
					t.Errorf("image change trigger %d should be disabled", i)
				}
			}
		}
	}
}

func hasStrategyDiff(a, b *deployapi.DeploymentConfig) bool {
	return a.Spec.Strategy.Type != b.Spec.Strategy.Type
}

func hasTriggerDiff(a, b *deployapi.DeploymentConfig) bool {
	if len(a.Spec.Triggers) != len(b.Spec.Triggers) {
		return true
	}

	for _, triggerA := range a.Spec.Triggers {
		bHasTrigger := false
		for _, triggerB := range b.Spec.Triggers {
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
	if a.Spec.Replicas != b.Spec.Replicas {
		return true
	}

	for keyA, valueA := range a.Spec.Selector {
		if valueB, exists := b.Spec.Selector[keyA]; !exists || valueA != valueB {
			return true
		}
	}

	return false
}

func hasPodTemplateDiff(a, b *deployapi.DeploymentConfig) bool {
	specA, specB := a.Spec.Template.Spec, b.Spec.Template.Spec
	return !kapi.Semantic.DeepEqual(specA, specB)
}
