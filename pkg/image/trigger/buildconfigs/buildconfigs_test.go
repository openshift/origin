package buildconfigs

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/diff"
	"k8s.io/apimachinery/pkg/util/sets"
	testingcore "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"

	buildapi "github.com/openshift/origin/pkg/build/apis/build"
	"github.com/openshift/origin/pkg/client/testclient"
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

func testBuildConfig(params []buildapi.ImageChangeTrigger) *buildapi.BuildConfig {
	obj := &buildapi.BuildConfig{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		Spec:       buildapi.BuildConfigSpec{},
	}
	for i := range params {
		obj.Spec.Triggers = append(obj.Spec.Triggers, buildapi.BuildTriggerPolicy{ImageChange: &params[i]})
	}
	return obj
}

func testBuildRequest(from *kapi.ObjectReference, core string, triggers map[string]string) *buildapi.BuildRequest {
	req := &buildapi.BuildRequest{
		ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
		From:       from,
	}
	keys := sets.NewString()
	for image := range triggers {
		keys.Insert(image)
	}
	for _, image := range keys.List() {
		id := triggers[image]
		req.TriggeredBy = append(req.TriggeredBy, buildapi.BuildTriggerCause{Message: "Image change", ImageChangeBuild: &buildapi.ImageChangeCause{ImageID: id, FromRef: &kapi.ObjectReference{Kind: "ImageStreamTag", Namespace: "other", Name: image}}})
	}
	if len(core) > 0 {
		req.TriggeredByImage = &kapi.ObjectReference{Kind: "DockerImage", Name: core}
	}

	return req
}

type instantiator struct {
	ns      string
	request *buildapi.BuildRequest
	build   *buildapi.Build
	err     error
}

func (i *instantiator) Instantiate(namespace string, request *buildapi.BuildRequest) (*buildapi.Build, error) {
	i.ns = namespace
	i.request = request
	return i.build, i.err
}

func TestBuildConfigReactor(t *testing.T) {
	testCases := []struct {
		tags        []fakeTagResponse
		obj         *buildapi.BuildConfig
		response    *buildapi.Build
		expected    *buildapi.BuildRequest
		expectedErr bool
	}{
		{
			obj: &buildapi.BuildConfig{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "default"},
			},
		},

		{
			// no container, last expected changed
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// no ref, no change
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
		},

		{
			// resolved without a change in another namespace
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will not resolve if not automatic
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if both triggers aren't resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &kapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if a trigger has already been resolved before
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From:                 &kapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
					LastTriggeredImageID: "old-image",
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},

		{
			// will fire if two identical triggers are resolved
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
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
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
				{
					From: &kapi.ObjectReference{Name: "stream-2:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1", "stream-2:1": "image-lookup-2"},
			),
		},

		{
			// will fire from single trigger
			tags: []fakeTagResponse{{Namespace: "other", Name: "stream-1:1", Ref: "image-lookup-1", RV: 2}},
			obj: testBuildConfig([]buildapi.ImageChangeTrigger{
				{
					From: &kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				},
			}),
			response: &buildapi.Build{},
			expected: testBuildRequest(
				&kapi.ObjectReference{Name: "stream-1:1", Namespace: "other", Kind: "ImageStreamTag"},
				"image-lookup-1",
				map[string]string{"stream-1:1": "image-lookup-1"},
			),
		},
	}

	for i, test := range testCases {
		c := &testclient.Fake{}
		var actualUpdate runtime.Object
		if test.response != nil {
			c.AddReactor("update", "*", func(action testingcore.Action) (handled bool, ret runtime.Object, err error) {
				actualUpdate = action.(testingcore.UpdateAction).GetObject()
				return true, test.response, nil
			})
		}
		instantiator := &instantiator{build: test.response}
		r := BuildConfigReactor{Instantiator: instantiator}
		initial, err := kapi.Scheme.DeepCopy(test.obj)
		if err != nil {
			t.Fatal(err)
		}
		err = r.ImageChanged(test.obj, fakeTagRetriever(test.tags))
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
