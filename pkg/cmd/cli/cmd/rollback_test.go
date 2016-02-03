package cmd

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	ktc "k8s.io/kubernetes/pkg/client/unversioned/testclient"

	deployapi "github.com/openshift/origin/pkg/deploy/api"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func TestRollbackOptions_findTargetDeployment(t *testing.T) {
	type existingDeployment struct {
		version int
		status  deployapi.DeploymentStatus
	}
	tests := []struct {
		name            string
		configVersion   int
		desiredVersion  int
		existing        []existingDeployment
		expectedVersion int
		errorExpected   bool
	}{
		{
			name:          "desired found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, deployapi.DeploymentStatusComplete},
				{2, deployapi.DeploymentStatusComplete},
				{3, deployapi.DeploymentStatusComplete},
			},
			desiredVersion:  1,
			expectedVersion: 1,
			errorExpected:   false,
		},
		{
			name:          "desired not found",
			configVersion: 3,
			existing: []existingDeployment{
				{2, deployapi.DeploymentStatusComplete},
				{3, deployapi.DeploymentStatusComplete},
			},
			desiredVersion: 1,
			errorExpected:  true,
		},
		{
			name:          "desired not supplied, target found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, deployapi.DeploymentStatusComplete},
				{2, deployapi.DeploymentStatusFailed},
				{3, deployapi.DeploymentStatusComplete},
			},
			desiredVersion:  0,
			expectedVersion: 1,
			errorExpected:   false,
		},
		{
			name:          "desired not supplied, target not found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, deployapi.DeploymentStatusFailed},
				{2, deployapi.DeploymentStatusFailed},
				{3, deployapi.DeploymentStatusComplete},
			},
			desiredVersion: 0,
			errorExpected:  true,
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)

		existingControllers := &kapi.ReplicationControllerList{}
		for _, existing := range test.existing {
			config := deploytest.OkDeploymentConfig(existing.version)
			deployment, _ := deployutil.MakeDeployment(config, kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
			deployment.Annotations[deployapi.DeploymentStatusAnnotation] = string(existing.status)
			existingControllers.Items = append(existingControllers.Items, *deployment)
		}

		fakekc := ktc.NewSimpleFake(existingControllers)
		opts := &RollbackOptions{
			kc: fakekc,
		}

		config := deploytest.OkDeploymentConfig(test.configVersion)
		target, err := opts.findTargetDeployment(config, test.desiredVersion)
		if err != nil {
			if !test.errorExpected {
				t.Fatalf("unexpected error: %s", err)
			}
			continue
		} else {
			if test.errorExpected && err == nil {
				t.Fatalf("expected an error")
			}
		}

		if target == nil {
			t.Fatalf("expected a target deployment")
		}
		if e, a := test.expectedVersion, deployutil.DeploymentVersionFor(target); e != a {
			t.Errorf("expected target version %d, got %d", e, a)
		}
	}
}
