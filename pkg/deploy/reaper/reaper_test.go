package reaper

import (
	"fmt"
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
		oc        *testclient.Fake
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
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
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
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(5)),
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
			oc:        testclient.NewSimpleFake(notfound()),
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
			oc:        testclient.NewSimpleFake(notfound()),
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
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(5)),
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
		reaper := &DeploymentConfigReaper{oc: test.oc, kc: test.kc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		out, err := reaper.Stop(test.namespace, test.name, nil)
		if !test.err && err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if test.err && err == nil {
			t.Errorf("%s: expected an error", test.testName)
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
		if out != test.output {
			t.Errorf("%s: unexpected output %q, expected %q", test.testName, out, test.output)
		}
	}
}

// TestDeploymentReaper_scenarios verifies DeploymentReaper functionality
// using table tests.
func TestDeploymentReaper_scenarios(t *testing.T) {
	type pod struct {
		name        string
		canBeReaped bool
	}
	scenarios := []struct {
		name                  string
		pods                  []pod
		deploymentCanBeReaped bool
	}{
		{
			name: "scenario 1",
			pods: []pod{
				{"pod1", true},
				{"pod2", true},
				{"pod3", false},
				{"pod4", true},
			},
			deploymentCanBeReaped: true,
		},
		{
			name: "scenario 2",
			pods: []pod{
				{"pod1", true},
				{"pod2", true},
				{"pod3", true},
				{"pod4", true},
			},
			deploymentCanBeReaped: true,
		},
		{
			name: "scenario 3",
			pods: []pod{
				{"pod1", false},
				{"pod2", false},
			},
			deploymentCanBeReaped: true,
		},
		{
			name: "scenario 4",
			pods: []pod{},
			deploymentCanBeReaped: true,
		},
		{
			name: "scenario 5",
			pods: []pod{},
			deploymentCanBeReaped: false,
		},
		{
			name: "scenario 6",
			pods: []pod{
				{"pod1", true},
				{"pod2", false},
			},
			deploymentCanBeReaped: false,
		},
	}

	for _, s := range scenarios {
		t.Logf("testing scenario: %s", s.name)
		reapedPods := []string{}
		reaper := &DeploymentReaper{
			deployerPodsFor: func(namespace, name string) (*kapi.PodList, error) {
				list := &kapi.PodList{}
				for _, pod := range s.pods {
					list.Items = append(list.Items, kapi.Pod{ObjectMeta: kapi.ObjectMeta{Name: pod.name}})
				}
				return list, nil
			},
			controllerReaper: &testReaper{
				stop: func(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
					if s.deploymentCanBeReaped {
						return name, nil
					}
					return name, fmt.Errorf("error stopping %s", name)
				},
			},
			podReaper: &testReaper{
				stop: func(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
					for _, pod := range s.pods {
						if pod.name == name && !pod.canBeReaped {
							return name, fmt.Errorf("error stopping %s", name)
						}
					}
					reapedPods = append(reapedPods, name)
					return name, nil
				},
			},
		}
		result, err := reaper.Stop("test", "frontend-1", &kapi.DeleteOptions{})
		t.Logf("Reaper output:\n%s", result)

		// Verify absence deployment reaping error
		if s.deploymentCanBeReaped && err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		// Verify expected deployment reaping error
		if !s.deploymentCanBeReaped && err == nil {
			t.Fatalf("expected an error")
		} else if !s.deploymentCanBeReaped && err != nil {
			t.Logf("got expected error: %v", err)
		}

		// Verify that expected pods were reaped.
		for _, pod := range s.pods {
			reaped := false
			for _, name := range reapedPods {
				if pod.name == name {
					reaped = true
				}
			}
			if pod.canBeReaped && !reaped {
				t.Errorf("expected %s to be reaped", pod.name)
			}
		}

		// Verify that no unexpected pods were reaped.
		for _, reaped := range reapedPods {
			expected := false
			for _, pod := range s.pods {
				if pod.name == reaped && pod.canBeReaped {
					expected = true
				}
			}
			if !expected {
				t.Errorf("unexpected reaping of pod %s", reaped)
			}
		}
	}
}

type testReaper struct {
	stop func(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error)
}

func (r *testReaper) Stop(namespace, name string, gracePeriod *kapi.DeleteOptions) (string, error) {
	return r.stop(namespace, name, gracePeriod)
}
