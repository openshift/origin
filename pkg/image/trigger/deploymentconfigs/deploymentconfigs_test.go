package deploymentconfigs

import (
	"reflect"
	"sort"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	testingcore "k8s.io/client-go/testing"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	appsv1 "github.com/openshift/api/apps/v1"
	appsclient "github.com/openshift/client-go/apps/clientset/versioned/fake"
)

type fakeTagResponse struct {
	Namespace string
	Name      string
	Ref       string
	RV        int64
}

type fakeTagRetriever []fakeTagResponse

func (r fakeTagRetriever) ImageStreamTag(namespace, name string) (string, int64, bool) {
	for _, resp := range r {
		if resp.Namespace != namespace || resp.Name != name {
			continue
		}
		return resp.Ref, resp.RV, true
	}
	return "", 0, false
}

func testDeploymentConfig(params []appsv1.DeploymentTriggerImageChangeParams, containers map[string]string) *appsv1.DeploymentConfig {
	obj := &appsv1.DeploymentConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec: appsv1.DeploymentConfigSpec{
			Template: &corev1.PodTemplateSpec{},
		},
	}
	for i := range params {
		obj.Spec.Triggers = append(obj.Spec.Triggers, appsv1.DeploymentTriggerPolicy{ImageChangeParams: &params[i]})
	}
	var names []string
	for k := range containers {
		names = append(names, k)
	}
	sort.Sort(sort.StringSlice(names))
	for _, name := range names {
		obj.Spec.Template.Spec.Containers = append(obj.Spec.Template.Spec.Containers, corev1.Container{Name: name, Image: containers[name]})
	}
	return obj
}

func TestDeploymentConfigReactor(t *testing.T) {
	testCases := []struct {
		tags        []fakeTagResponse
		obj         *appsv1.DeploymentConfig
		response    *appsv1.DeploymentConfig
		expected    *appsv1.DeploymentConfig
		expectedErr bool
	}{
		{
			obj: &appsv1.DeploymentConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
		},

		{
			// no container, last expected changed
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
			}, nil),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, nil),
		},

		{
			// no container, second run
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-2", RV: 3}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, nil),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-2",
				},
			}, nil),
		},

		{
			// no ref, no change
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
			}, map[string]string{"test": ""}),
		},

		{
			// resolved without a change in another namespace
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
			}, map[string]string{"test": ""}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "image-lookup-1"}),
		},

		{
			// will not resolve if not automatic
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      false,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
			}, map[string]string{"test": ""}),
			response: &appsv1.DeploymentConfig{},
		},

		{
			// will not fire if both triggers aren't resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test2"},
				},
			}, map[string]string{"test": "", "test2": ""}),
			response: &appsv1.DeploymentConfig{},
		},

		{
			// will fire if a trigger has already been resolved before
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "old-image",
				},
			}, map[string]string{"test": "", "test2": "old-image"}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "old-image",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "old-image"}),
		},

		{
			// will fire if the same trigger has already been resolved before
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-2:1", Ref: "image-lookup-2", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "old-image",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "old-image", "test2": "image-lookup-1"}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "old-image",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-2",
				},
			}, map[string]string{"test": "old-image", "test2": "image-lookup-2"}),
		},

		{
			// will not fire the image can't be resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-2:1", Ref: "image-lookup-2", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
			}, map[string]string{"test": "", "test2": "image-lookup-1"}),
			response: &appsv1.DeploymentConfig{},
			expected: nil,
		},

		{
			// will not fire the one image can't be resolved and the other can
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-2:1", Ref: "image-lookup-2", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": ""}),
			response: &appsv1.DeploymentConfig{},
			expected: nil,
		},

		{
			// will fire if both triggers are resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test"},
				},
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test2"},
				},
			}, map[string]string{"test": "", "test2": ""}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "image-lookup-1"}),
		},

		{
			// will fire if both triggers are resolved, second run
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-2", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-1",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "image-lookup-1"}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test"},
					LastTriggeredImage: "image-lookup-2",
				},
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test2"},
					LastTriggeredImage: "image-lookup-2",
				},
			}, map[string]string{"test": "image-lookup-2", "test2": "image-lookup-2"}),
		},

		{
			// will fire from single trigger
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:      true,
					From:           corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames: []string{"test", "test2"},
				},
			}, map[string]string{"test": "", "test2": ""}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test", "test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "image-lookup-1"}),
		},

		{
			// will fire from single trigger, second run
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-2", RV: 2}},
			obj: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test", "test2"},
					LastTriggeredImage: "image-lookup-1",
				},
			}, map[string]string{"test": "image-lookup-1", "test2": "image-lookup-1"}),
			response: &appsv1.DeploymentConfig{},
			expected: testDeploymentConfig([]appsv1.DeploymentTriggerImageChangeParams{
				{
					Automatic:          true,
					From:               corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					ContainerNames:     []string{"test", "test2"},
					LastTriggeredImage: "image-lookup-2",
				},
			}, map[string]string{"test": "image-lookup-2", "test2": "image-lookup-2"}),
		},
	}

	for _, test := range testCases {
		t.Run("", func(t *testing.T) {
			c := &appsclient.Clientset{}
			var actualUpdate runtime.Object
			if test.response != nil {
				c.AddReactor("update", "*", func(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
					actualUpdate = action.(testingcore.UpdateAction).GetObject()
					return true, test.response, nil
				})
			}
			r := DeploymentConfigReactor{Client: c.Apps()}
			initial := test.obj.DeepCopy()
			err := r.ImageChanged(test.obj, fakeTagRetriever(test.tags))
			if !kapihelper.Semantic.DeepEqual(initial, test.obj) {
				t.Errorf("should not have mutated: %s", diff.ObjectReflectDiff(initial, test.obj))
			}
			switch {
			case err == nil && test.expectedErr, err != nil && !test.expectedErr:
				t.Fatalf("unexpected error: %v", err)
			case err != nil:
				return
			}
			if test.expected != nil {
				actions := c.Actions()
				if len(actions) != 1 || actions[0].GetVerb() != "update" {
					t.Fatalf("unexpected actions: %v", actions)
				}
				if actualUpdate == nil {
					t.Fatalf("no response defined %#v", actions)
				}
				if !reflect.DeepEqual(test.expected, actualUpdate) {
					t.Fatalf("not equal: %s", diff.ObjectReflectDiff(test.expected, actualUpdate))
				}
			} else {
				if len(c.Actions()) != 0 {
					t.Fatalf("unexpected actions: %v", c.Actions())
				}
			}
		})
	}
}
