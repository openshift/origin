package topology_transitions

import (
	"testing"

	configv1 "github.com/openshift/api/config/v1"
)

func TestTransitionTargetFromEnvironmentDefaultsHACompactToFalse(t *testing.T) {
	environment := map[string]string{
		targetControlPlaneTopologyEnvVar:   string(configv1.HighlyAvailableTopologyMode),
		targetInfrastructureTopologyEnvVar: string(configv1.HighlyAvailableTopologyMode),
	}
	getenv := func(name string) string { return environment[name] }

	target, err := transitionTargetFromEnvironment(getenv)
	if err != nil {
		t.Fatalf("transitionTargetFromEnvironment() error = %v", err)
	}
	if target.HACompact {
		t.Fatal("expected HA compact to default to false when its environment variable is unset")
	}
}
