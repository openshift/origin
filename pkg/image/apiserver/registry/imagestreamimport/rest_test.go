package imagestreamimport

import (
	"context"
	"errors"
	"testing"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/util/validation/field"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kapihelper "k8s.io/kubernetes/pkg/apis/core/helper"

	"github.com/openshift/api/image"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

type fakeImageCreater struct{}

func (_ fakeImageCreater) New() runtime.Object {
	return nil
}

func (_ fakeImageCreater) Create(ctx context.Context, obj runtime.Object, _ rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	return obj, nil
}

func TestImportSuccessful(t *testing.T) {
	one := int64(1)
	two := int64(2)
	now := metav1.Now()
	tests := map[string]struct {
		image                       *imageapi.Image
		stream                      *imageapi.ImageStream
		importReferencePolicyType   imageapi.TagReferencePolicyType
		expectedTagEvent            imageapi.TagEvent
		expectedReferencePolicyType imageapi.TagReferencePolicyType
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
			expectedTagEvent: imageapi.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
			importReferencePolicyType:   imageapi.SourceTagReferencePolicy,
			expectedReferencePolicyType: imageapi.SourceTagReferencePolicy,
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
			expectedTagEvent: imageapi.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
			importReferencePolicyType:   imageapi.LocalTagReferencePolicy,
			expectedReferencePolicyType: imageapi.LocalTagReferencePolicy,
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
							ReferencePolicy: imageapi.TagReferencePolicy{
								Type: imageapi.SourceTagReferencePolicy,
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{},
			},
			expectedTagEvent: imageapi.TagEvent{
				Created:              now,
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
			importReferencePolicyType:   imageapi.LocalTagReferencePolicy,
			expectedReferencePolicyType: imageapi.SourceTagReferencePolicy,
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
							ReferencePolicy: imageapi.TagReferencePolicy{
								Type: imageapi.LocalTagReferencePolicy,
							},
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
			expectedTagEvent: imageapi.TagEvent{
				DockerImageReference: "registry.com/namespace/image:mytag",
				Image:                "image",
				Generation:           two,
			},
			importReferencePolicyType:   imageapi.SourceTagReferencePolicy,
			expectedReferencePolicyType: imageapi.LocalTagReferencePolicy,
		},
	}

	for name, test := range tests {
		ref, err := imageapi.ParseDockerImageReference(test.image.DockerImageReference)
		if err != nil {
			t.Errorf("%s: error parsing image ref: %v", name, err)
			continue
		}

		importPolicy := imageapi.TagImportPolicy{}
		referencePolicy := imageapi.TagReferencePolicy{Type: test.importReferencePolicyType}
		importedImages := make(map[string]error)
		updatedImages := make(map[string]*imageapi.Image)
		storage := REST{images: fakeImageCreater{}}
		_, ok := storage.importSuccessful(apirequest.NewDefaultContext(), test.image, test.stream,
			ref.Tag, ref.Exact(), two, now, importPolicy, referencePolicy, importedImages, updatedImages)
		if !ok {
			t.Errorf("%s: expected success, didn't get one", name)
		}
		actual := test.stream.Status.Tags[ref.Tag].Items[0]
		if !kapihelper.Semantic.DeepEqual(actual, test.expectedTagEvent) {
			t.Errorf("%s: expected %#v, got %#v", name, test.expectedTagEvent, actual)
		}

		actualRefType := test.stream.Spec.Tags["mytag"].ReferencePolicy.Type
		if actualRefType != test.expectedReferencePolicyType {
			t.Errorf("%s: expected %#v, got %#v", name, test.expectedReferencePolicyType, actualRefType)
		}
	}
}

// errMessageString is a part of error message copied from quotaAdmission.Admit() method in
// k8s.io/kubernetes/plugin/pkg/admission/resourcequota/admission.go module
const errQuotaMessageString = `exceeded quota:`
const errQuotaUnknownMessageString = `status unknown for quota:`
const errLimitsMessageString = `exceeds the maximum limit`

// TestIsErrorLimitExceeded tests for limit errors.
func TestIsErrorLimitExceeded(t *testing.T) {
	for _, tc := range []struct {
		name        string
		err         error
		shouldMatch bool
	}{
		{
			name: "unrelated error",
			err:  errors.New("unrelated"),
		},
		{
			name: "wrong type",
			err:  errors.New(errQuotaMessageString),
		},
		{
			name: "wrong kapi type",
			err:  kapierrors.NewUnauthorized(errQuotaMessageString),
		},
		{
			name: "unrelated forbidden error",
			err:  kapierrors.NewForbidden(kapi.Resource("imageStreams"), "is", errors.New("unrelated")),
		},
		{
			name: "unrelated invalid error",
			err: kapierrors.NewInvalid(image.Kind("imageStreams"), "is",
				field.ErrorList{
					field.Required(field.NewPath("imageStream").Child("Spec"), "detail"),
				}),
		},
		{
			name: "quota error not recognized with invalid reason",
			err: kapierrors.NewInvalid(image.Kind("imageStreams"), "is",
				field.ErrorList{
					field.Forbidden(field.NewPath("imageStreams"), errQuotaMessageString),
				}),
		},
		{
			name: "quota unknown error not recognized with invalid reason",
			err: kapierrors.NewInvalid(image.Kind("imageStreams"), "is",
				field.ErrorList{
					field.Forbidden(field.NewPath("imageStreams"), errQuotaUnknownMessageString),
				}),
		},
		{
			name: "quota exceeded error",
			err:  kapierrors.NewForbidden(kapi.Resource("imageStream"), "is", errors.New(errQuotaMessageString)),
		},
		{
			name: "quota unknown error",
			err:  kapierrors.NewForbidden(kapi.Resource("imageStream"), "is", errors.New(errQuotaUnknownMessageString)),
		},
		{
			name:        "limits exceeded error with forbidden reason",
			err:         kapierrors.NewForbidden(image.Resource("imageStream"), "is", errors.New(errLimitsMessageString)),
			shouldMatch: true,
		},
		{
			name: "limits exceeded error with invalid reason",
			err: kapierrors.NewInvalid(image.Kind("imageStreams"), "is",
				field.ErrorList{
					field.Forbidden(field.NewPath("imageStream"), errLimitsMessageString),
				}),
			shouldMatch: true,
		},
	} {
		match := isErrorLimitExceeded(tc.err)

		if !match && tc.shouldMatch {
			t.Errorf("[%s] expected to match error [%T]: %v", tc.name, tc.err, tc.err)
		}
		if match && !tc.shouldMatch {
			t.Errorf("[%s] expected not to match error [%T]: %v", tc.name, tc.err, tc.err)
		}
	}
}
