package cmd

import (
	"reflect"
	"testing"
	"time"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"
	"k8s.io/kubernetes/pkg/client/testing/core"
	"k8s.io/kubernetes/pkg/labels"
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
	var (
		deploymentConfigsResource      = unversioned.GroupVersionResource{Resource: "deploymentconfigs"}
		replicationControllersResource = unversioned.GroupVersionResource{Resource: "replicationcontrollers"}
	)

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
		kc        *fake.Clientset
		expected  []core.Action
		kexpected []core.Action
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["simple-stop"])),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "legacy simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", nil),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["multi-stop"])),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "legacy stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", nil),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-2"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-3"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-4"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-5"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "no config, some deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"})}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewUpdateAction(replicationControllersResource, "default", nil),
				core.NewGetAction(replicationControllersResource, "default", "config-1"),
				core.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "no config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
			},
			err: true,
		},
		{
			testName:  "config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["no-deployments"])),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
			},
			err: false,
		},
		{
			testName:  "legacy config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []core.Action{
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewUpdateAction(deploymentConfigsResource, "default", nil),
				core.NewGetAction(deploymentConfigsResource, "default", "config"),
				core.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []core.Action{
				core.NewListAction(replicationControllersResource, "default", kapi.ListOptions{}),
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
			case core.UpdateAction:
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
			case core.GetAction, core.DeleteAction:
				if !reflect.DeepEqual(e, a) {
					t.Errorf("%s: unexpected action[%d]: %s, expected %s", test.testName, j, a, e)
				}
			}
		}
	}
}
