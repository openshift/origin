package deploymentconfigs

import (
	"reflect"
	"testing"
	"time"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/diff"
	clientgotesting "k8s.io/client-go/testing"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset/fake"

	appsapi "github.com/openshift/origin/pkg/apps/apis/apps"
	_ "github.com/openshift/origin/pkg/apps/apis/apps/install"
	appstest "github.com/openshift/origin/pkg/apps/apis/apps/test"
	appsfake "github.com/openshift/origin/pkg/apps/generated/internalclientset/fake"
	appsutil "github.com/openshift/origin/pkg/apps/util"
)

func mkdeployment(version int64) kapi.ReplicationController {
	deployment, _ := appsutil.MakeDeployment(appstest.OkDeploymentConfig(version), legacyscheme.Codecs.LegacyCodec(appsapi.SchemeGroupVersion))
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
		deploymentConfigsResource      = schema.GroupVersionResource{Group: "apps.openshift.io", Resource: "deploymentconfigs"}
		replicationControllersResource = schema.GroupVersionResource{Resource: "replicationcontrollers"}
		replicationControllerKind      = schema.GroupVersionKind{Kind: "ReplicationController"}
	)

	pauseBytes := []byte(`{"spec":{"paused":true,"replicas":0,"revisionHistoryLimit":0}}`)

	fakeDC := map[string]*appsapi.DeploymentConfig{
		"simple-stop":           appstest.OkDeploymentConfig(1),
		"legacy-simple-stop":    appstest.OkDeploymentConfig(1),
		"multi-stop":            appstest.OkDeploymentConfig(5),
		"legacy-multi-stop":     appstest.OkDeploymentConfig(5),
		"no-deployments":        appstest.OkDeploymentConfig(5),
		"legacy-no-deployments": appstest.OkDeploymentConfig(5),
	}

	emptyClientset := func() *appsfake.Clientset {
		result := &appsfake.Clientset{}
		result.AddReactor("patch", "deploymentconfigs", func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
			return true, nil, kapierrors.NewNotFound(schema.GroupResource{Group: "apps.openshift.io", Resource: "deploymentconfig"}, "config")
		})
		return result
	}

	tests := []struct {
		testName  string
		namespace string
		name      string
		oc        *appsfake.Clientset
		kc        *fake.Clientset
		expected  []clientgotesting.Action
		kexpected []clientgotesting.Action
		err       bool
	}{
		{
			testName:  "simple stop",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
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
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "legacy simple stop",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["legacy-simple-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
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
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
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
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "legacy stop multiple controllers",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["legacy-multi-stop"]),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1, 2, 3, 4, 5)),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
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
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-2"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-3"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-4"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-5"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-5"),
			},
			err: false,
		},
		{
			testName:  "no config, some deployments",
			namespace: "default",
			name:      "config",
			oc:        emptyClientset(),
			kc:        fake.NewSimpleClientset(mkdeploymentlist(1)),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{}),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewUpdateAction(replicationControllersResource, "default", nil),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewGetAction(replicationControllersResource, "default", "config-1"),
				clientgotesting.NewDeleteAction(replicationControllersResource, "default", "config-1"),
			},
			err: false,
		},
		{
			testName:  "no config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        emptyClientset(),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
			},
			err: true,
		},
		{
			testName:  "config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
			},
			err: false,
		},
		{
			testName:  "legacy config, no deployments",
			namespace: "default",
			name:      "config",
			oc:        appsfake.NewSimpleClientset(fakeDC["legacy-no-deployments"]),
			kc:        fake.NewSimpleClientset(&kapi.ReplicationControllerList{}),
			expected: []clientgotesting.Action{
				clientgotesting.NewPatchAction(deploymentConfigsResource, "default", "config", pauseBytes),
				clientgotesting.NewGetAction(deploymentConfigsResource, "default", "config"),
				clientgotesting.NewDeleteAction(deploymentConfigsResource, "default", "config"),
			},
			kexpected: []clientgotesting.Action{
				clientgotesting.NewListAction(replicationControllersResource, replicationControllerKind, "default", metav1.ListOptions{LabelSelector: labels.SelectorFromSet(map[string]string{"openshift.io/deployment-config.name": "config"}).String()}),
			},
			err: false,
		},
	}

	for _, test := range tests {
		reaper := &DeploymentConfigReaper{appsClient: test.oc, kc: test.kc, pollInterval: time.Millisecond, timeout: time.Millisecond}
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
			if j >= len(test.expected) {
				break
			}
			e, a := test.expected[j], actualAction
			switch a.(type) {
			case clientgotesting.UpdateAction:
				if e.GetVerb() != a.GetVerb() ||
					e.GetNamespace() != a.GetNamespace() ||
					e.GetResource() != a.GetResource() ||
					e.GetSubresource() != a.GetSubresource() {
					t.Errorf("%s: unexpected action[%d]: expected %s, got %s", test.testName, j, e, a)
				}
			default:
				if !reflect.DeepEqual(a, e) {
					t.Errorf("%s: unexpected action[%d]: %s", test.testName, j, diff.ObjectReflectDiff(a, e))
				}
			}
		}

		if len(test.oc.Actions()) < len(test.expected) {
			t.Errorf("%s: missing expected actions:", test.testName)
			for _, action := range test.expected[len(test.oc.Actions()):] {
				t.Errorf("\t%s\n", action)
			}
			continue
		} else if len(test.oc.Actions()) > len(test.expected) {
			t.Errorf("%s: unexpected additional actions:", test.testName)
			for _, action := range test.oc.Actions()[len(test.expected):] {
				t.Errorf("\t%s\n", action)
			}
			continue
		}

		for j, actualAction := range test.kc.Actions() {
			if j >= len(test.kexpected) {
				break
			}
			e, a := test.kexpected[j], actualAction
			switch a.(type) {
			case clientgotesting.UpdateAction:
				if e.GetVerb() != a.GetVerb() ||
					e.GetNamespace() != a.GetNamespace() ||
					e.GetResource() != a.GetResource() ||
					e.GetSubresource() != a.GetSubresource() {
					t.Errorf("%s: unexpected action[%d]: expected %s, got %s", test.testName, j, e, a)
				}
			default:
				if !reflect.DeepEqual(a, e) {
					t.Errorf("%s: unexpected action[%d]: %s", test.testName, j, diff.ObjectReflectDiff(a, e))
				}
			}
		}
		if len(test.kc.Actions()) < len(test.kexpected) {
			t.Errorf("%s: missing expected actions:", test.testName)
			for _, action := range test.kexpected[len(test.kc.Actions()):] {
				t.Errorf("\t%s\n", action)
			}
			continue
		} else if len(test.kc.Actions()) > len(test.kexpected) {
			t.Errorf("%s: unexpected additional actions:", test.testName)
			for _, action := range test.kc.Actions()[len(test.kexpected):] {
				t.Errorf("\t%s\n", action)
			}
			continue
		}
	}
}
