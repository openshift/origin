package scaler

import (
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/kubectl"

	"github.com/openshift/origin/pkg/client/testclient"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func mkdeployment(version int) kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codec)
	return *deployment
}

func mkDeploymentList(versions ...int) *kapi.ReplicationControllerList {
	list := &kapi.ReplicationControllerList{}
	for _, v := range versions {
		list.Items = append(list.Items, mkdeployment(v))
	}
	return list
}

func TestScale(t *testing.T) {
	tests := []struct {
		testName               string
		namespace              string
		name                   string
		count                  uint
		preconditions          *kubectl.ScalePrecondition
		retry, waitForReplicas *kubectl.RetryParams
		oc                     *testclient.Fake
		kc                     *ktestclient.Fake
		expected               []string
		kexpected              []string
		expectedErr            error
	}{
		{
			testName:  "simple scale",
			namespace: "default",
			name:      "foo",
			count:     uint(3),
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
			kc:        ktestclient.NewSimpleFake(mkDeploymentList(1)),
			expected: []string{
				"get-deploymentconfig",
			},
			kexpected: []string{
				"get-replicationController",
				"update-replicationController",
			},
			expectedErr: nil,
		},
		{
			testName:        "wait for replicas",
			namespace:       "default",
			name:            "foo",
			count:           uint(3),
			waitForReplicas: &kubectl.RetryParams{Interval: time.Millisecond, Timeout: time.Millisecond},
			oc:              testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
			kc:              ktestclient.NewSimpleFake(mkDeploymentList(1)),
			expected: []string{
				"get-deploymentconfig",
				"get-deploymentconfig",
			},
			kexpected: []string{
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"get-replicationController",
			},
			expectedErr: nil,
		},
	}

	for _, test := range tests {
		scaler := DeploymentConfigScaler{NewScalerClient(test.oc, test.kc)}
		got := scaler.Scale("default", test.name, test.count, test.preconditions, test.retry, test.waitForReplicas)
		if got != test.expectedErr {
			t.Errorf("%s: error mismatch: expected %v, got %v", test.expectedErr, got)
		}
		if len(test.oc.Actions) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, test.oc.Actions, test.expected)
		}
		for j, fake := range test.oc.Actions {
			if fake.Action != test.expected[j] {
				t.Errorf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.expected[j])
			}
		}
		if len(test.kc.Actions) != len(test.kexpected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, test.kc.Actions, test.kexpected)
		}
		for j, fake := range test.kc.Actions {
			if fake.Action != test.kexpected[j] {
				t.Errorf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.kexpected[j])
			}
		}
	}
}
