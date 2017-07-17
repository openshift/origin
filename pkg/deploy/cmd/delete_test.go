package cmd

import (
	"reflect"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	"github.com/openshift/origin/pkg/client/testclient"
	deployapi "github.com/openshift/origin/pkg/deploy/apis/apps"
	_ "github.com/openshift/origin/pkg/deploy/apis/apps/install"
	deploytest "github.com/openshift/origin/pkg/deploy/apis/apps/test"
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
		deploymentConfigsResource      = schema.GroupVersionResource{Resource: "deploymentconfigs"}
		replicationControllersResource = schema.GroupVersionResource{Resource: "replicationcontrollers"}
		replicationControllerKind      = schema.GroupVersionKind{Kind: "ReplicationController"}
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
		expected  []clientgotesting.Action
		kexpected []clientgotesting.Action
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["simple-stop"])),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "legacy simple stop",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", nil),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["multi-stop"])),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "legacy stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", nil),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "no config, some deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "no config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
			},
			err: true,
		},
		{
			testName:  "config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", pause(fakeDC["no-deployments"])),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
			},
			err: false,
		},
		{
			testName:  "legacy config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        testclient.NewSimpleFake(fakeDC["legacy-no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewUpdateAction(deploymentConfigsResource, "default", nil),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
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
			case clientgotesting.UpdateAction:
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
			case clientgotesting.GetAction, clientgotesting.DeleteAction:
				if !reflect.DeepEqual(e, a) {
					t.Errorf("%s: unexpected action[%d]: %s, expected %s", test.testName, j, a, e)
				}
			}
		}
	}
}
