package reaper

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/runtime"

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
		expected  []ktestclient.Action
		kexpected []ktestclient.Action
		output    string
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(deploytest.OkDeploymentConfig(1)),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []ktestclient.Action{
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
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
			expected: []ktestclient.Action{
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-5"),
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
			expected: []ktestclient.Action{
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", nil, nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
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
			expected: []ktestclient.Action{
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", nil, nil),
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
			expected: []ktestclient.Action{
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", nil, nil),
			},
			output: "config stopped",
			err:    false,
		},
	}

	for _, test := range tests {
		reaper := &DeploymentConfigReaper{oc: test.oc, kc: test.kc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		out, err := reaper.Stop(test.namespace, test.name, 1*time.Second, nil)

		if !test.err && err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if test.err && err == nil {
			t.Errorf("%s: expected an error", test.testName)
		}
		if len(test.oc.Actions()) != len(test.expected) {
			t.Fatalf("%s: unexpected actions: %v, expected %v", test.testName, test.oc.Actions, test.expected)
		}
		for j, actualAction := range test.oc.Actions() {
			if !reflect.DeepEqual(actualAction, test.expected[j]) {
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
		if out != test.output {
			t.Errorf("%s: unexpected output %q, expected %q", test.testName, out, test.output)
		}
	}
}
