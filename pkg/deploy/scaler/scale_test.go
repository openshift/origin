package scaler

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	ktestclient "k8s.io/kubernetes/pkg/client/testclient"
	"k8s.io/kubernetes/pkg/kubectl"

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
		expected               []ktestclient.Action
		kexpected              []ktestclient.Action
		expectedErr            error
	}{
		{
			testName:  "simple scale",
			namespace: "default",
			name:      "foo",
			count:     uint(3),
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
			kc:        ktestclient.NewSimpleFake(mkDeploymentList(1)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "foo"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewGetAction("replicationcontrollers", "default", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "default", nil),
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
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "foo"),
				ktestclient.NewGetAction("deploymentconfigs", "default", "foo"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewGetAction("replicationcontrollers", "default", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "default", nil),
				ktestclient.NewGetAction("replicationcontrollers", "default", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
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
		if len(test.oc.Actions()) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, test.oc.Actions(), test.expected)
		}
		for j, actualAction := range test.oc.Actions() {
			if actualAction != test.expected[j] {
				t.Errorf("%s: unexpected action: %s, expected %s", test.testName, actualAction, test.expected[j])
			}
		}
		if len(test.kc.Actions()) != len(test.kexpected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, test.kc.Actions(), test.kexpected)
		}
		for j, actualAction := range test.kc.Actions() {
			e, a := test.kexpected[j], actualAction
			if e.GetVerb() != a.GetVerb() ||
				e.GetNamespace() != a.GetNamespace() ||
				e.GetResource() != a.GetResource() ||
				e.GetSubresource() != a.GetSubresource() {
				t.Errorf("%s: unexpected action[%d]: %s, expected %s", test.testName, j, a, e)
			}

			switch a.(type) {
			case ktestclient.GetAction, ktestclient.DeleteAction:
				if !reflect.DeepEqual(e, a) {
					t.Errorf("%s: unexpected action[%d]: %s, expected %s", test.testName, j, a, e)
				}
			}
		}
	}
}
