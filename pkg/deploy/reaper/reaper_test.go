package reaper

import (
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	kerrors "github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func mkdeployment(version int) kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codec)
	return *deployment
}

func mkdeploymentlist(versions ...int) *kapi.ReplicationControllerList {
	list := &kapi.ReplicationControllerList{}
	for _, v := range versions {
		list.Items = append(list.Items, mkdeployment(v))
	}
	return list
}

func TestStop(t *testing.T) {
	notfound := func() runtime.Object {
		return &(kerrors.NewNotFound("DeploymentConfig", "config").(*kerrors.StatusError).ErrStatus)
	}

	tests := []struct {
		testName  string
		namespace string
		name      string
		osc       *testclient.Fake
		kc        *ktestclient.Fake
		expected  []string
		kexpected []string
		output    string
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			osc:       testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []string{
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
			},
			output: "config stopped",
			err:    false,
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "config",
			osc:       testclient.NewSimpleFake(deploytest.OkDeploymentConfig(5)),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []string{
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
			},
			output: "config stopped",
			err:    false,
		},
		{
			testName:  "no config, some deployments",
			namespace: "default",
			name:      "config",
			osc:       testclient.NewSimpleFake(notfound()),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []string{
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
			},
			output: "config stopped",
			err:    false,
		},
		{
			testName:  "no config, no deployments",
			namespace: "default",
			name:      "config",
			osc:       testclient.NewSimpleFake(notfound()),
			kc:        ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{}),
			expected: []string{
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
			},
			output: "",
			err:    true,
		},
		{
			testName:  "config, no deployments",
			namespace: "default",
			name:      "config",
			osc:       testclient.NewSimpleFake(deploytest.OkDeploymentConfig(5)),
			kc:        ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{}),
			expected: []string{
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
			},
			output: "config stopped",
			err:    false,
		},
	}

	for _, test := range tests {
		reaper := &DeploymentConfigReaper{osc: test.osc, kc: test.kc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		out, err := reaper.Stop(test.namespace, test.name, nil)
		if !test.err && err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if test.err && err == nil {
			t.Errorf("%s: expected an error", test.testName)
		}
		if len(test.osc.Actions) != len(test.expected) {
			t.Errorf("%s: unexpected actions: %v, expected %v", test.testName, test.osc.Actions, test.expected)
		}
		for j, fake := range test.osc.Actions {
			if fake.Action != test.expected[j] {
				t.Errorf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.expected[j])
			}
		}
		if len(test.kc.Actions) != len(test.kexpected) {
			t.Errorf("%s: unexpected actions: %v, expected %v", test.testName, test.kc.Actions, test.kexpected)
		}
		for j, fake := range test.kc.Actions {
			if fake.Action != test.kexpected[j] {
				t.Errorf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.kexpected[j])
			}
		}
		if out != test.output {
			t.Errorf("%s: unexpected output %q, expected %q", test.testName, out, test.output)
		}
	}
}
