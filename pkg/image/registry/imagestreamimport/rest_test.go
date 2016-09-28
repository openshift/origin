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

func TestCheckImportFailure(t *testing.T) {
	testNow := unversioned.Now()
	nonzero := int64(100)
	zero := int64(0)
	tests := map[string]struct {
		status         api.ImageImportStatus
		stream         *api.ImageStream
		tag            string
		returnExpected bool
		streamExpected *api.ImageStream
	}{
		"check success": {
			status: api.ImageImportStatus{
				Image: &api.Image{
					DockerImageReference: "registry.com/namespace/image:mytag",
				},
				Status: unversioned.Status{
					Status: unversioned.StatusSuccess,
				},
				Tag: "mytag",
			},
			stream:         &api.ImageStream{},
			returnExpected: false,
			streamExpected: &api.ImageStream{},
		},
		"check failure without setting tagConditions and tagRef": {
			status: api.ImageImportStatus{
				Image: &api.Image{
					DockerImageReference: "registry.com/namespace/image",
				},
				Status: unversioned.Status{
					Status: unversioned.StatusFailure,
				},
				Tag: "",
			},
			stream:         &api.ImageStream{},
			tag:            "",
			returnExpected: true,
			streamExpected: &api.ImageStream{},
		},
		"check failure with setting tagConditions and tagRef": {
			status: api.ImageImportStatus{
				Image: &api.Image{
					DockerImageReference: "registry.com/namespace/image:mytag",
				},
				Status: unversioned.Status{
					Status: unversioned.StatusFailure,
					Reason: "reason",
				},
				Tag: "",
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
							Generation: &nonzero,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"mytag": {
							Items: []api.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "image",
							}},
						},
					},
				},
			},
			tag:            "",
			returnExpected: true,
			streamExpected: &api.ImageStream{
				Spec: api.ImageStreamSpec{
					Tags: map[string]api.TagReference{
						"mytag": {
							Name: "mytag",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: "registry.com/namespace/image:mytag",
							},
							Generation: &zero,
						},
					},
				},
				Status: api.ImageStreamStatus{
					Tags: map[string]api.TagEventList{
						"mytag": {
							Items: []api.TagEvent{{
								DockerImageReference: "registry.com/namespace/image:othertag",
								Image:                "image",
							}},
							Conditions: []api.TagEventCondition{{
								Type:               api.ImportSuccess,
								Status:             kapi.ConditionFalse,
								Message:            "unknown error prevented import",
								Reason:             "reason",
								Generation:         0,
								LastTransitionTime: testNow,
							}},
						},
					},
				},
			},
		},
	}

	for name, test := range tests {
		ref := checkImportFailure(test.status, test.stream, test.tag, int64(0), testNow)
		if !kapi.Semantic.DeepEqual(ref, test.returnExpected) {
			t.Errorf("%s: returnExpected %#v, got %#v", name, test.returnExpected, ref)
		}

		if !kapi.Semantic.DeepEqual(test.stream, test.streamExpected) {
			t.Errorf("%s: streamExpected %#v, got %#v", name, test.streamExpected, test.stream)
		}
	}
}
