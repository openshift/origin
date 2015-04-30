package v1beta1

import (
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api/v1beta3"

	newer "github.com/openshift/origin/pkg/deploy/api"
)

func TestStrategyRoundTrip(t *testing.T) {
	p := DeploymentStrategy{
		Type:      DeploymentStrategyTypeRecreate,
		Resources: kapi.ResourceRequirements{},
	}
	out := &newer.DeploymentStrategy{}
	if err := api.Scheme.Convert(&p, out); err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}
