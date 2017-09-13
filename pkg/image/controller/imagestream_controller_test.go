package controller

import (
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"
	kcontroller "k8s.io/kubernetes/pkg/controller"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageclient "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"

	_ "github.com/openshift/origin/pkg/api/install"
)

func TestHandleImageStream(t *testing.T) {
	two := int64(2)
	testCases := []struct {
		stream *imageapi.ImageStream
		run    bool
	}{
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
			},
		},
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: "a random error"},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imageapi.ImageStreamSpec{
					DockerImageRepository: "test/other",
				},
			},
		},

		// references are ignored
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From:      &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Reference: true,
						},
					},
				},
			},
		},
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
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
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
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
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
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
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{Name: "test", Namespace: "other"},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{"latest": {Items: []imageapi.TagEvent{{Generation: 1}}}},
				},
			},
		},
		// spec tag with generation with status condition error and equal generation will not be imported
		{
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{"latest": {Conditions: []imageapi.TagEventCondition{
						{
							Type:       imageapi.ImportSuccess,
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
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{"latest": {Conditions: []imageapi.TagEventCondition{
						{
							Type:       imageapi.ImportSuccess,
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
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
					Name:        "test",
					Namespace:   "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"latest": {
							From:       &kapi.ObjectReference{Kind: "DockerImage", Name: "test/other:latest"},
							Generation: &two,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{"latest": {Items: []imageapi.TagEvent{{Generation: 1}}}},
				},
			},
		},
		// test external repo
		{
			run: true,
			stream: &imageapi.ImageStream{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test",
					Namespace: "other",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
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
		fake := imageclient.NewSimpleClientset()
		other, err := kapi.Scheme.DeepCopy(test.stream)
		if err != nil {
			t.Fatal(err)
		}

		if err := handleImageStream(test.stream, fake.Image(), nil); err != nil {
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
			if !kapihelper.Semantic.DeepEqual(test.stream, other) {
				t.Errorf("%d: did not expect change to stream: %s", i, diff.ObjectGoPrintDiff(test.stream, other))
			}
			if len(fake.Actions()) != 0 {
				t.Errorf("%d: did not expect remote calls", i)
			}
		}
	}
}

func TestProcessNextWorkItemOnRemovedStream(t *testing.T) {
	clientset := imageclient.NewSimpleClientset()
	informer := imageinformer.NewSharedInformerFactory(imageclient.NewSimpleClientset(), 0)
	isc := NewImageStreamController(clientset, informer.Image().InternalVersion().ImageStreams())
	isc.queue.Add("other/test")
	isc.processNextWorkItem()
	if isc.queue.Len() != 0 {
		t.Errorf("Unexpected queue length, expected 0, got %d", isc.queue.Len())
	}
}

func TestProcessNextWorkItem(t *testing.T) {
	stream := &imageapi.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Annotations: map[string]string{imageapi.DockerImageRepositoryCheckAnnotation: metav1.Now().UTC().Format(time.RFC3339)},
			Name:        "test",
			Namespace:   "other",
		},
	}
	clientset := imageclient.NewSimpleClientset(stream)
	informer := imageinformer.NewSharedInformerFactory(imageclient.NewSimpleClientset(stream), 0)
	isc := NewImageStreamController(clientset, informer.Image().InternalVersion().ImageStreams())
	key, _ := kcontroller.KeyFunc(stream)
	isc.queue.Add(key)
	isc.processNextWorkItem()
	if isc.queue.Len() != 0 {
		t.Errorf("Unexpected queue length, expected 0, got %d", isc.queue.Len())
	}
}
