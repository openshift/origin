package image

import (
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	kquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client/testclient"
	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

const maxTestImportTagsPerRepository = 5

func TestImageStreamImportEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imageapi.ImageStream
		isiSpec         imageapi.ImageStreamImportSpec
		expectedISCount int64
	}{
		{
			name: "nothing to import",
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
			},
		},

		{
			name: "dry run",
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: false,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
		},

		{
			name: "wrong from kind",
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "test",
						Name:      imageapi.MakeImageStreamImageName("someis", imagetest.BaseImageWith1LayerDigest),
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import from repository to empty project",
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/fedora",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import from repository to existing image stream",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
				},
			},
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
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
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "spec",
					},
				},
			},
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "import images",
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{
					{
						From: kapi.ObjectReference{
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
			isiSpec: imageapi.ImageStreamImportSpec{
				Import: true,
				Images: []imageapi.ImageImportSpec{
					{
						From: kapi.ObjectReference{
							Kind: "DockerImage",
							Name: "docker.io/centos:latest",
						},
					},
				},
				Repository: &imageapi.RepositoryImportSpec{
					From: kapi.ObjectReference{
						Kind: "DockerImage",
						Name: "docker.io/library/fedora",
					},
				},
			},
			expectedISCount: 1,
		},
	} {

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, tc.iss...))

		evaluator := NewImageStreamImportEvaluator(fakeClient)

		isi := &imageapi.ImageStreamImport{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "test",
				Name:      "is",
			},
			Spec: tc.isiSpec,
		}

		usage := evaluator.Usage(isi)
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
