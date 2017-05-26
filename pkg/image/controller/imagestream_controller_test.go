package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/api"

	client "github.com/openshift/origin/pkg/client/testclient"
	"github.com/openshift/origin/pkg/image/api"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestHandleImageStream(t *testing.T) {
	two := int64(2)
	testCases := []struct {
		stream *api.ImageStream
		run    bool
	}{
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: "a random error"},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},

		// references are ignored
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:      &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:      &kapi.ObjectReference{Kind: "AnotherImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},

		// spec tag will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From: &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
						},
					},
				},
			},
		},
		// spec tag with generation with no pending status will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Items: []api.TagEvent{{Generation: 1}}}},
				},
			},
		},
		// spec tag with generation with status condition error and equal generation will not be imported
		{
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Conditions: []api.TagEventCondition{
						{
							Type:       api.ImportSuccess,
							Status:     kapi.ConditionFalse,
							Generation: 2,
						},
					}}},
				},
			},
		},
		// spec tag with generation with status condition error and older generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Conditions: []api.TagEventCondition{
						{
							Type:       api.ImportSuccess,
							Status:     kapi.ConditionFalse,
							Generation: 1,
						},
					}}},
				},
			},
		},
		// spec tag with generation with older status generation will be imported
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{api.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{"latest": {Items: []api.TagEvent{{Generation: 1}}}},
				},
			},
		},
		// test external repo
		{
			run: true,
			stream: &api.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "other",
				},
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"1.1": {
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "some/repo:mytag",
							},
						},
					},
				},
			},
		},
	}

	for i, test := range testCases {
		fake := &client.Fake{}
		other, err := kapi.Scheme.DeepCopy(test.stream)
		if err != nil {
			t.Fatal(err)
		}

		if err := handleImageStream(test.stream, fake, nil); err != nil {
			t.Errorf("%d: unexpected error: %v", i, err)
		}
		if test.run {
			actions := fake.Actions()
			if len(actions) == 0 {
				t.Errorf("%d: expected remote calls: %#v", i, fake)
				continue
			}
			if !actions[0].Matches("create", "imagestreamimports") {
				t.Errorf("expected a create action: %#v", actions)
			}
		} else {
			if !kapi.Semantic.DeepEqual(test.stream, other) {
				t.Errorf("%d: did not expect change to stream: %s", i, diff.ObjectGoPrintDiff(test.stream, other))
			}
			if len(fake.Actions()) != 0 {
				t.Errorf("%d: did not expect remote calls", i)
			}
		}
	}
}
