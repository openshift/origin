package buildconfigs

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	buildv1 "github.com/openshift/api/build/v1"
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

func testBuildConfig(params []buildv1.ImageChangeTrigger) *buildv1.BuildConfig {
	obj := &buildv1.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       buildv1.BuildConfigSpec{},
	}
	for i := range params {
		obj.Spec.Triggers = append(obj.Spec.Triggers, buildv1.BuildTriggerPolicy{ImageChange: &params[i]})
	}
	return obj
}

func testBuildRequest(from *corev1.ObjectReference, core string, triggers map[string]string) *buildv1.BuildRequest {
	req := &buildv1.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		From:       from,
	}
	keys := sets.NewString()
	for image := range triggers {
		keys.Insert(image)
	}
	for _, image := range keys.List() {
		id := triggers[image]
		req.TriggeredBy = append(req.TriggeredBy, buildv1.BuildTriggerCause{Message: "Image change", ImageChangeBuild: &buildv1.ImageChangeCause{ImageID: id, FromRef: &corev1.ObjectReference{Kind: "ImageStreamTag", Namespace: "other", Name: image}}})
	}
	if len(core) > 0 {
		req.TriggeredByImage = &corev1.ObjectReference{Kind: "DockerImage", Name: core}
	}

	return req
}

type instantiator struct {
	ns      string
	request *buildv1.BuildRequest
	build   *buildv1.Build
	err     error
}

func (i *instantiator) Instantiate(namespace string, request *buildv1.BuildRequest) (*buildv1.Build, error) {
	i.ns = namespace
	i.request = request
	return i.build, i.err
}

func TestBuildConfigReactor(t *testing.T) {
	testCases := []struct {
		tags        []fakeTagResponse
		obj         *buildv1.BuildConfig
		response    *buildv1.Build
		expected    *buildv1.BuildRequest
		expectedErr bool
	}{
		{
			obj: &buildv1.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
		},

		{
			// no container, last expected changed
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// no ref, no change
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
		},

		{
			// resolved without a change in another namespace
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will not resolve if not automatic
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if both triggers aren't resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if a trigger has already been resolved before
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From:                 &corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					LastTriggeredImageID: "old-image",
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if two identical triggers are resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if multiple triggers are resolved
			tags: []fakeTagResponse{
				{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2},
				{Namespace: "other", Name: "stream-2:1", Ref: "image-lookup-2", RV: 2},
			},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1", "stream-2:1": "image-lookup-2"},
			),
		},

		{
			// will fire from single trigger
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From: &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// won't fire because it is paused
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From:   &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					Paused: true,
				},
			}),
		},

		{
			// will fire only for unpaused if multiple triggers are resolved
			tags: []fakeTagResponse{
				{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2},
				{Namespace: "other", Name: "stream-2:1", Ref: "image-lookup-2", RV: 2},
			},
			obj: testBuildConfig([]buildv1.ImageChangeTrigger{
				{
					From:   &corev1.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
					Paused: true,
				},
				{
					From: &corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildv1.Build{},
			expected: testBuildRequest(
				&corev1.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-2",
				map[string]string{"stream-2:1": "image-lookup-2"},
			),
		},
	}

	for i, test := range testCases {
		instantiator := &instantiator{build: test.response}
		r := buildConfigReactor{instantiator: instantiator}
		initial := test.obj.DeepCopy()
		err := r.ImageChanged(test.obj, fakeTagRetriever(test.tags))
		if !kapihelper.Semantic.DeepEqual(initial, test.obj) {
			t.Errorf("%d: should not have mutated: %s", i, diff.ObjectReflectDiff(initial, test.obj))
		}
		switch {
		case err == nil && test.expectedErr, err != nil && !test.expectedErr:
			t.Errorf("%d: unexpected error: %v", i, err)
			continue
		case err != nil:
			continue
		}
		if test.expected != nil {
			if instantiator.request == nil {
				t.Errorf("%d: unexpected request: %v", i, instantiator.request)
			}
			if !reflect.DeepEqual(test.expected, instantiator.request) {
				t.Errorf("%d: not equal: %s", i, diff.ObjectReflectDiff(test.expected, instantiator.request))
				t.Logf("%#v", instantiator.request.TriggeredBy)
				continue
			}
		} else {
			if instantiator.request != nil {
				t.Errorf("%d: unexpected request: %v", i, instantiator.request)
			}
		}
	}
}
