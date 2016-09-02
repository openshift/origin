package imagestreamimport

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
)

type fakeImageCreater struct{}

func (_ fakeImageCreater) New() runtime.Object {
	return nil
}

func (_ fakeImageCreater) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	return obj, nil
}

func TestImportSuccessful(t *testing.T) {
	one := int64(1)
	two := int64(2)
	now := unversioned.Now()
	tests := map[string]struct {
		image    *api.Image
		stream   *api.ImageStream
		expected api.TagEvent
	}{
		"reference differs": {
			image: &api.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &api.ImageStream{
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"mytag": {
							Items: []api.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: api.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		"image differs": {
			image: &api.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &api.ImageStream{
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"mytag": {
							Items: []api.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "non-image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: api.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		"empty status": {
			image: &api.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &api.ImageStream{
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
				Status: api.ImageStreamStatus{},
			},
			expected: api.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
		// https://github.com/openshift/origin/issues/10402:
		"only generation differ": {
			image: &api.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name: "image",
				},
				DockerImageReference: "registry.com/namespace/image:mytag",
			},
			stream: &api.ImageStream{
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
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
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"mytag": {
							Items: []api.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:mytag",
								Image:                "image",
								Generation:           one,
							}},
						},
					},
				},
			},
			expected: api.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
		},
	}

	for name, test := range tests {
		ref, err := api.ParseDockerImageReference(test.image.DockerImageReference)
		if err != nil {
			t.Errorf("%s: error parsing image ref: %v", name, err)
			continue
		}

		importPolicy := api.TagImportPolicy{}
		importedImages := make(map[string]error)
		updatedImages := make(map[string]*api.Image)
		storage := REST{images: fakeImageCreater{}}
		_, ok := storage.importSuccessful(kapi.NewDefaultContext(), test.image, test.stream,
			ref.Tag, ref.Exact(), two, now, importPolicy, importedImages, updatedImages)
		if !ok {
			t.Errorf("%s: expected success, didn't get one", name)
		}
		actual := test.stream.Status.Tags[ref.Tag].Items[0]
		if !kapi.Semantic.DeepEqual(actual, test.expected) {
			t.Errorf("%s: expected %#v, got %#v", name, test.expected, actual)
		}
	}
}
