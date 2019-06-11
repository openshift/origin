package image

import (
	"fmt"
	"testing"

	corev1 "k8s.io/api/core/v1"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientgotesting "k8s.io/client-go/testing"
	kapi "k8s.io/kubernetes/pkg/apis/core"
	kquota "k8s.io/kubernetes/pkg/quota/v1"

	imagev1 "github.com/openshift/api/image/v1"
	fakeimagev1client "github.com/openshift/client-go/image/clientset/versioned/fake"
	imagev1informer "github.com/openshift/client-go/image/informers/externalversions"
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
						Name:      "is@" + MiscImageDigest,
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
						Name:      "is@" + MiscImageDigest,
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
						Name:        MiscImageDigest,
						Annotations: map[string]string{imagev1.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: MakeDockerImageReference("shared", "is", MiscImageDigest),
					DockerImageManifest:  MiscImageDigest,
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
						Name:        MiscImageDigest,
						Annotations: map[string]string{imagev1.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: MakeDockerImageReference("test", "dest", MiscImageDigest),
					DockerImageManifest:  MiscImage,
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
										DockerImageReference: MakeDockerImageReference("test", "havingtag", BaseImageWith1LayerDigest),
										Image:                BaseImageWith1LayerDigest,
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
						Name:      "is@" + ChildImageWith2LayersDigest,
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
		fakeClient.AddReactor("get", "imagestreams", GetFakeImageStreamGetHandler(t, tc.iss...))
		imageInformers := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().V1().ImageStreams()
		for _, is := range tc.iss {
			isInformer.Informer().GetIndexer().Add(&is)
		}
		evaluator := NewImageStreamTagEvaluator(isInformer.Lister(), fakeClient.ImageV1())

		usage, err := evaluator.Usage(&tc.ist)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}

		expectedUsage := expectedResourceListFor(tc.expectedISCount)
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

// InternalRegistryURL is an url of internal docker registry for testing purposes.
const InternalRegistryURL = "172.30.12.34:5000"

// MakeDockerImageReference makes a docker image reference string referencing testing internal docker
// registry.
func MakeDockerImageReference(ns, isName, imageID string) string {
	return fmt.Sprintf("%s/%s/%s@%s", InternalRegistryURL, ns, isName, imageID)
}

// GetFakeImageStreamGetHandler creates a test handler to be used as a reactor with  core.Fake client
// that handles Get request on image stream resource. Matching is from given image stream list will be
// returned if found. Additionally, a shared image stream may be requested.
func GetFakeImageStreamGetHandler(t *testing.T, iss ...imagev1.ImageStream) clientgotesting.ReactionFunc {
	sharedISs := []imagev1.ImageStream{*GetSharedImageStream("shared", "is")}
	allISs := append(sharedISs, iss...)

	return func(action clientgotesting.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case clientgotesting.GetAction:
			for _, is := range allISs {
				if is.Namespace == a.GetNamespace() && a.GetName() == is.Name {
					t.Logf("imagestream get handler: returning image stream %s/%s", is.Namespace, is.Name)
					return true, &is, nil
				}
			}

			err := kerrors.NewNotFound(kapi.Resource("imageStreams"), a.GetName())
			t.Logf("imagestream get handler: %v", err)
			return true, nil, err
		}
		return false, nil, nil
	}
}

// GetSharedImageStream returns an image stream having all the testing images tagged in its status under
// latest tag.
func GetSharedImageStream(namespace, name string) *imagev1.ImageStream {
	tevList := []imagev1.TagEvent{}
	for _, imgName := range []string{
		BaseImageWith1LayerDigest,
		BaseImageWith2LayersDigest,
		ChildImageWith2LayersDigest,
		ChildImageWith3LayersDigest,
		MiscImageDigest,
	} {
		tevList = append(tevList,
			imagev1.TagEvent{
				DockerImageReference: MakeDockerImageReference("test", "is", imgName),
				Image:                imgName,
			})
	}

	sharedIS := imagev1.ImageStream{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imagev1.ImageStreamStatus{
			Tags: []imagev1.NamedTagEventList{
				{
					Tag:   "latest",
					Items: tevList,
				},
			},
		},
	}

	return &sharedIS
}
