package image

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kquota "k8s.io/kubernetes/pkg/quota"

	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imagefake "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
	imageinternal "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

func TestImageStreamTagEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imageapi.ImageStream
		ist             imageapi.ImageStreamTag
		expectedISCount int64
	}{
		{
			name: "empty image stream",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imageapi.ImageStreamStatus{},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imageapi.TagReference{
					Name: "dest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "shared",
						Name:      "is@" + imagetest.MiscImageDigest,
					},
				},
			},
			expectedISCount: 0,
		},

		{
			name: "no image stream",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imageapi.TagReference{
					Name: "dest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "shared",
						Name:      "is@" + imagetest.MiscImageDigest,
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "no image stream using image stream tag",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imageapi.TagReference{
					Name: "dest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "shared",
						Name:      "is:latest",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "no tag given",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name:        imagetest.MiscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: imagetest.MakeDockerImageReference("shared", "is", imagetest.MiscImageDigest),
					DockerImageManifest:  imagetest.MiscImageDigest,
				},
			},
			expectedISCount: 1,
		},

		{
			name: "missing from",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imageapi.TagReference{
					Name: "dest",
				},
				Image: imageapi.Image{
					ObjectMeta: metav1.ObjectMeta{
						Name:        imagetest.MiscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: imagetest.MakeDockerImageReference("test", "dest", imagetest.MiscImageDigest),
					DockerImageManifest:  imagetest.MiscImage,
				},
			},
			expectedISCount: 1,
		},

		{
			name: "update existing tag",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: imagetest.MakeDockerImageReference("test", "havingtag", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
								},
							},
						},
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "havingtag:latest",
				},
				Tag: &imageapi.TagReference{
					Name: "latest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "shared",
						Name:      "is@" + imagetest.ChildImageWith2LayersDigest,
					},
				},
			},
			expectedISCount: 0,
		},

		{
			name: "add a new tag with 2 image streams",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "other",
						Name:      "is2",
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "destis:latest",
				},
				Tag: &imageapi.TagReference{
					Name: "latest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "other",
						Name:      "is2:latest",
					},
				},
			},
			expectedISCount: 1,
		},
	} {
		fakeClient := &imagefake.Clientset{}
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, tc.iss...))
		imageInformers := imageinformer.NewSharedInformerFactory(imageinternal.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().InternalVersion().ImageStreams()
		for _, is := range tc.iss {
			isInformer.Informer().GetIndexer().Add(&is)
		}
		evaluator := NewImageStreamTagEvaluator(isInformer.Lister(), fakeClient.Image())

		usage, err := evaluator.Usage(&tc.ist)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedUsage := imagetest.ExpectedResourceListFor(tc.expectedISCount)
		expectedResources := kquota.ResourceNames(expectedUsage)
		if len(usage) != len(expectedUsage) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(usage), len(expectedResources))
		}

		masked := kquota.Mask(usage, expectedResources)
		if len(masked) != len(expectedUsage) {
			for k := range usage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: got unexpected resource %q from Usage() method", tc.name, k)
				}
			}

			for k := range expectedUsage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: expected resource %q not computed", tc.name, k)
				}
			}
		}

		for rname, expectedValue := range expectedUsage {
			if v, exists := masked[rname]; exists {
				if v.Cmp(expectedValue) != 0 {
					t.Errorf("[%s]: got unexpected usage for %q: %s != %s", tc.name, rname, v.String(), expectedValue.String())
				}
			}
		}
	}
}
