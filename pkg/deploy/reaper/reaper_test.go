package reaper

import (
	"testing"
	"time"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	ktestclient "github.com/GoogleCloudPlatform/kubernetes/pkg/client/testclient"

	"github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/deploy/api"
)

func TestStop(t *testing.T) {
	fakeClients := []*testclient.Fake{
		testclient.NewSimpleFake(&api.DeploymentConfig{
			LatestVersion: 1,
		}),
		testclient.NewSimpleFake(&api.DeploymentConfig{
			LatestVersion: 5,
		}),
	}
	fakeKClients := []*ktestclient.Fake{
		ktestclient.NewSimpleFake(&kapi.ReplicationController{}),
		ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{
			Items: []kapi.ReplicationController{
				{},
				{},
				{},
				{},
				{},
			},
		}),
	}

	tests := []struct {
		testName  string
		namespace string
		name      string
		osc       *testclient.Fake
		kc        *ktestclient.Fake
		reaper    *DeploymentConfigReaper
		expected  []string
		kexpected []string
		output    string
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "foo",
			osc:       fakeClients[0],
			kc:        fakeKClients[0],
			reaper:    &DeploymentConfigReaper{osc: fakeClients[0], kc: fakeKClients[0], pollInterval: time.Millisecond, timeout: time.Millisecond},
			expected: []string{
				"get-deploymentconfig",
				"update-deploymentconfig",
				"delete-deploymentconfig",
			},
			kexpected: []string{
				"list-replicationControllers",
				"get-replicationController",
				"update-replicationController",
				"get-replicationController",
				"delete-replicationController",
			},
			output: "foo stopped",
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "foo",
			osc:       fakeClients[1],
			kc:        fakeKClients[1],
			reaper:    &DeploymentConfigReaper{osc: fakeClients[1], kc: fakeKClients[1], pollInterval: time.Millisecond, timeout: time.Millisecond},
			expected: []string{
				"get-deploymentconfig",
				"update-deploymentconfig",
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
			output: "foo stopped",
		},
	}

	if len(fakeClients) != len(tests) {
		t.Fatalf("no. of clients should equal to the no. of tests. Fix those tests.")
	}

	for i, test := range tests {
		out, err := test.reaper.Stop(test.namespace, test.name, nil)
		if err != nil {
			t.Fatalf("%s: unexpected error: %v", test.testName, err)
		}
		if len(fakeClients[i].Actions) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, fakeClients[i].Actions, test.expected)
		}
		for j, fake := range fakeClients[i].Actions {
			if fake.Action != test.expected[j] {
				t.Fatalf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.expected[j])
			}
		}
		if len(fakeKClients[i].Actions) != len(test.kexpected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, fakeKClients[i].Actions, test.kexpected)
		}
		for j, fake := range fakeKClients[i].Actions {
			if fake.Action != test.kexpected[j] {
				t.Fatalf("%s: unexpected action: %s, expected %s", test.testName, fake.Action, test.kexpected[j])
			}
		}
		if out != test.output {
			t.Fatalf("%s: unexpected output %q, expected %q", test.testName, out, test.output)
		}
	}
}
