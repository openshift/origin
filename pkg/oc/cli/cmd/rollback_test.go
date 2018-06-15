package cmd

import (
	"testing"

	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

func TestRollbackOptions_findTargetDeployment(t *testing.T) {
	type existingDeployment struct {
		version int64
		status  appsapi.DeploymentStatus
	}
	tests := []struct {
		name            string
		configVersion   int64
		desiredVersion  int64
		existing        []existingDeployment
		expectedVersion int64
		errorExpected   bool
	}{
		{
			name:          "desired found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, appsapi.DeploymentStatusComplete},
				{2, appsapi.DeploymentStatusComplete},
				{3, appsapi.DeploymentStatusComplete},
			},
			desiredVersion:  1,
			expectedVersion: 1,
			errorExpected:   false,
		},
		{
			name:          "desired not found",
			configVersion: 3,
			existing: []existingDeployment{
				{2, appsapi.DeploymentStatusComplete},
				{3, appsapi.DeploymentStatusComplete},
			},
			desiredVersion: 1,
			errorExpected:  true,
		},
		{
			name:          "desired not supplied, target found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, appsapi.DeploymentStatusComplete},
				{2, appsapi.DeploymentStatusFailed},
				{3, appsapi.DeploymentStatusComplete},
			},
			desiredVersion:  0,
			expectedVersion: 1,
			errorExpected:   false,
		},
		{
			name:          "desired not supplied, target not found",
			configVersion: 3,
			existing: []existingDeployment{
				{1, appsapi.DeploymentStatusFailed},
				{2, appsapi.DeploymentStatusFailed},
				{3, appsapi.DeploymentStatusComplete},
			},
			desiredVersion: 0,
			errorExpected:  true,
		},
	}

	for _, test := range tests {
		t.Logf("evaluating test: %s", test.name)

		existingControllers := &kapi.ReplicationControllerList{}
		for _, existing := range test.existing {
			config := appstest.OkDeploymentConfig(existing.version)
			deployment, _ := appsutil.MakeDeployment(config, legacyscheme.Codecs.LegacyCodec(appsapi.SchemeGroupVersion))
			deployment.Annotations[appsapi.DeploymentStatusAnnotation] = string(existing.status)
			existingControllers.Items = append(existingControllers.Items, *deployment)
		}

		fakekc := fake.NewSimpleClientset(existingControllers)
		opts := &RollbackOptions{
			kc: fakekc,
		}

		config := appstest.OkDeploymentConfig(test.configVersion)
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
		if e, a := test.expectedVersion, appsutil.DeploymentVersionFor(target); e != a {
			t.Errorf("expected target version %d, got %d", e, a)
		}
	}
}
