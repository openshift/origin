package imagestreamimport

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	kapi "k8s.io/kubernetes/pkg/api"
	kapihelper "k8s.io/kubernetes/pkg/api/helper"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type fakeImageCreater struct{}

func (_ fakeImageCreater) New() runtime.Object {
	return nil
}

func (_ fakeImageCreater) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	return obj, nil
}

func TestImportSuccessful(t *testing.T) {
	one := int64(1)
	two := int64(2)
	now := metav1.Now()
	tests := map[string]struct {
		image    *imageapi.Image
		stream   *imageapi.ImageStream
		expected imageapi.TagEvent
	}{
		"reference differs": {
			image: &imageapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							Name: "mytag",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "registry.com/namespace/image:mytag",
							},
							Generation: &one,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"mytag": {
							Items: []imageapi.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: imageapi.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		"image differs": {
			image: &imageapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							Name: "mytag",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "registry.com/namespace/image:mytag",
							},
							Generation: &one,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"mytag": {
							Items: []imageapi.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "non-image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: imageapi.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		"empty status": {
			image: &imageapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							Name: "mytag",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "registry.com/namespace/image:mytag",
							},
							Generation: &one,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{},
			},
			expected: imageapi.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		// https://github.com/openshift/origin/issues/10402:
		"only generation differ": {
			image: &imageapi.Image{
				ObjectMeta: metav1.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"mytag": {
							Name: "mytag",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "registry.com/namespace/image:mytag",
							},
							Generation: &two,
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"mytag": {
							Items: []imageapi.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:mytag",
								Image:                "image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: imageapi.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
	}

	for name, test := range tests {
		ref, err := imageapi.ParseDockerImageReference(test.image.DockerImageReference)
		if err != nil {
			t.Errorf("%s: error parsing image ref: %v", name, err)
			continue
		}

		importPolicy := imageapi.TagImportPolicy{}
		referencePolicy := imageapi.TagReferencePolicy{Type: imageapi.SourceTagReferencePolicy}
		importedImages := make(map[string]error)
		updatedImages := make(map[string]*imageapi.Image)
		storage := REST{images: fakeImageCreater{}}
		_, ok := storage.importSuccessful(apirequest.NewDefaultContext(), test.image, test.stream,
			ref.Tag, ref.Exact(), two, now, importPolicy, referencePolicy, importedImages, updatedImages)
		if !ok {
			t.Errorf("%s: expected success, didn't get one", name)
		}
		actual := test.stream.Status.Tags[ref.Tag].Items[0]
		if !kapihelper.Semantic.DeepEqual(actual, test.expected) {
			t.Errorf("%s: expected %#v, got %#v", name, test.expected, actual)
		}
	}
}
