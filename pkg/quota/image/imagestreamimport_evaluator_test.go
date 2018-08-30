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

const maxTestImportTagsPerRepository = 5

func TestImageStreamImportEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imagev1.ImageStream
		isiSpec         imagev1.ImageStreamImportSpec
		expectedISCount int64
	}{
		{
			name: "nothing to import",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
			},
		},

		{
			name: "dry run",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: false,
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
		},

		{
			name: "wrong from kind",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "test",
						Name:      imageapi.JoinImageStreamImage("someis", imagetest.BaseImageWith1LayerDigest),
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import from repository to empty project",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/fedora",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import from repository to existing image stream",
			iss: []imagev1.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
				},
			},
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/fedora",
					},
				},
			},
			// target image stream already exists
			expectedISCount: 0,
		},

		{
			name: "import from repository to non-empty project",
			iss: []imagev1.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "spec",
					},
				},
			},
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import images",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "docker.io/library/fedora:f23",
						},
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import image and repository",
			isiSpec: imagev1.ImageStreamImportSpec{
				Import: true,
				Images: []imagev1.ImageImportSpec{
					{
						From: corev1.ObjectReference{
							Kind: "DockerImage",
							Name: "docker.io/centos:latest",
						},
					},
				},
				Repository: &imagev1.RepositoryImportSpec{
					From: corev1.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
			expectedISCount: 1,
		},
	} {
		imageInformers := imagev1informer.NewSharedInformerFactory(fakeimagev1client.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().V1().ImageStreams()
		for _, is := range tc.iss {
			isInformer.Informer().GetIndexer().Add(&is)
		}
		evaluator := NewImageStreamImportEvaluator(isInformer.Lister())

		isi := &imagev1.ImageStreamImport{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "is",
			},
			Spec: tc.isiSpec,
		}

		usage, err := evaluator.Usage(isi)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
		expectedUsage := imagetest.ExpectedResourceListFor(tc.expectedISCount)
		expectedResources := kquota.ResourceNames(expectedUsage)
		if len(usage) != len(expectedResources) {
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
