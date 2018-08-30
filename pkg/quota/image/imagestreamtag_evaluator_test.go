package image

import (
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kquota "k8s.io/kubernetes/pkg/quota"

	imagev1 "github.com/openshift/api/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imagetest "github.com/openshift/origin/pkg/image/util/testutil"
)

func TestImageStreamTagEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imagev1.ImageStream
		ist             imagev1.ImageStreamTag
		expectedISCount int64
	}{
		{
			name: "empty image stream",
			iss: []imagev1.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imagev1.ImageStreamStatus{},
				},
			},
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imagev1.TagReference{
					Name: "dest",
					From: &corev1.ObjectReference{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imagev1.TagReference{
					Name: "dest",
					From: &corev1.ObjectReference{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imagev1.TagReference{
					Name: "dest",
					From: &corev1.ObjectReference{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Image: imagev1.Image{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imagev1.TagReference{
					Name: "dest",
				},
				Image: imagev1.Image{
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
			iss: []imagev1.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
					Status: imagev1.ImageStreamStatus{
						Tags: []imagev1.NamedTagEventList{
							{
								Tag: "latest",
								Items: []imagev1.TagEvent{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "havingtag:latest",
				},
				Tag: &imagev1.TagReference{
					Name: "latest",
					From: &corev1.ObjectReference{
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
			iss: []imagev1.ImageStream{
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
			ist: imagev1.ImageStreamTag{
				ObjectMeta: metav1.ObjectMeta{
					Namespace: "test",
					Name:      "destis:latest",
				},
				Tag: &imagev1.TagReference{
					Name: "latest",
					From: &corev1.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "other",
						Name:      "is2:latest",
					},
				},
			},
			expectedISCount: 1,
		},
	} {
		fakeClient := fakeimagev1client.NewSimpleClientset()
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, tc.iss...))
		imageInformers := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().V1().ImageStreams()
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
