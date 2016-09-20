package cmd

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/api"
	_ "github.com/openshift/origin/pkg/deploy/api/install"
	deploytest "github.com/openshift/origin/pkg/deploy/api/test"
	deployutil "github.com/openshift/origin/pkg/deploy/util"
)

func mkdeployment(version int64) kapi.ReplicationController {
	deployment, _ := deployutil.MakeDeployment(deploytest.OkDeploymentConfig(version), kapi.Codecs.LegacyCodec(deployapi.SchemeGroupVersion))
	return *deployment
}

func mkdeploymentlist(versions ...int64) *kapi.ReplicationControllerList {
	list := &kapi.ReplicationControllerList{}
	for _, v := range versions {
		list.Items = append(list.Items, mkdeployment(v))
	}
	return list
}

func TestStop(t *testing.T) {
	notfound := func() runtime.Object {
		return &(kerrors.NewNotFound(kapi.Resource("DeploymentConfig"), "config").ErrStatus)
	}

	pause := func(d *deployapi.DeploymentConfig) *deployapi.DeploymentConfig {
		d.Spec.Paused = true
		return d
	}

	fakeDC := map[string]*deployapi.DeploymentConfig{
		"simple-stop":           deploytest.OkDeploymentConfig(1),
		"legacy-simple-stop":    deploytest.OkDeploymentConfig(1),
		"multi-stop":            deploytest.OkDeploymentConfig(5),
		"legacy-multi-stop":     deploytest.OkDeploymentConfig(5),
		"no-deployments":        deploytest.OkDeploymentConfig(5),
		"legacy-no-deployments": deploytest.OkDeploymentConfig(5),
	}

	tests := []struct {
		testName  string
		namespace string
		name      string
		oc        *testclient.Fake
		kc        *ktestclient.Fake
		expected  []ktestclient.Action
		kexpected []ktestclient.Action
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["simple-stop"]),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", pause(fakeDC["simple-stop"])),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
			},
			err: false,
		},
		{
			testName:  "legacy simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-simple-stop"]),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", nil),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
			},
			err: false,
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["multi-stop"]),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", pause(fakeDC["multi-stop"])),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-5"),
			},
			err: false,
		},
		{
			testName:  "legacy stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-multi-stop"]),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", nil),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-2"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-3"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-4"),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-5"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-5"),
			},
			err: false,
		},
		{
			testName:  "no config, some deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(notfound()),
			kc:        ktestclient.NewSimpleFake(mkdeploymentlist(1)),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewListAction("replicationcontrollers", "", kapi.ListOptions{}),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewUpdateAction("replicationcontrollers", "", nil),
				ktestclient.NewGetAction("replicationcontrollers", "", "config-1"),
				ktestclient.NewDeleteAction("replicationcontrollers", "", "config-1"),
			},
			err: false,
		},
		{
			testName:  "no config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(notfound()),
			kc:        ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{}),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{}),
			},
			err: true,
		},
		{
			testName:  "config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["no-deployments"]),
			kc:        ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{}),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", pause(fakeDC["no-deployments"])),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{}),
			},
			err: false,
		},
		{
			testName:  "legacy config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-no-deployments"]),
			kc:        ktestclient.NewSimpleFake(&kapi.ReplicationControllerList{}),
			expected: []ktestclient.Action{
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewUpdateAction("deploymentconfigs", "default", nil),
				ktestclient.NewGetAction("deploymentconfigs", "default", "config"),
				ktestclient.NewDeleteAction("deploymentconfigs", "default", "config"),
			},
			kexpected: []ktestclient.Action{
				ktestclient.NewListAction("replicationcontrollers", "default", kapi.ListOptions{}),
			},
			err: false,
		},
	}

	for _, test := range tests {
		reaper := &DeploymentConfigReaper{oc: test.oc, kc: test.kc, pollInterval: time.Millisecond, timeout: time.Millisecond}
		err := reaper.Stop(test.namespace, test.name, 1*time.Second, nil)

		if !test.err && err != nil {
			t.Errorf("%s: unexpected error: %v", test.testName, err)
		}
		if test.err && err == nil {
			t.Errorf("%s: expected an error", test.testName)
		}
		if len(test.oc.Actions()) != len(test.expected) {
			t.Errorf("%s: unexpected actions: %s", test.testName, diff.ObjectReflectDiff(test.oc.Actions(), test.expected))
			continue
		}
		for j, actualAction := range test.oc.Actions() {
			e, a := test.expected[j], actualAction
			switch a.(type) {
			case ktestclient.UpdateAction:
				if e.GetVerb() != a.GetVerb() ||
					e.GetNamespace() != a.GetNamespace() ||
					e.GetResource() != a.GetResource() ||
					e.GetSubresource() != a.GetSubresource() {
					t.Errorf("%s: unexpected action[%d]: %s, expected %s", test.testName, j, a, e)
				}
			default:
				if !reflect.DeepEqual(actualAction, test.expected[j]) {
					t.Errorf("%s: unexpected action: %s", test.testName, diff.ObjectReflectDiff(actualAction, test.expected[j]))
				}
			}
		}
		if len(test.kc.Actions()) != len(test.kexpected) {
			t.Errorf("%s: unexpected actions: %s", test.testName, diff.ObjectReflectDiff(test.kc.Actions(), test.kexpected))
			continue
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
