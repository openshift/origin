package image

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
	imagetest "github.com/openshift/origin/pkg/quota/image/testutil"
)

func TestImageStreamTagEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		iss            []imageapi.ImageStream
		ist            imageapi.ImageStreamTag
		expectedImages int64
	}{
		{
			name: "empty image stream",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imageapi.ImageStreamStatus{},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
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
			expectedImages: 1,
		},

		{
			name: "no image stream",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
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
			expectedImages: 1,
		},

		{
			name: "no image stream using image stream tag",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
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
			expectedImages: 1,
		},

		{
			name: "no tag given",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Image: imageapi.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:        imagetest.MiscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", "shared", "is", imagetest.MiscImageDigest),
					DockerImageManifest:  imagetest.MiscImageDigest,
				},
			},
			expectedImages: 0,
		},

		{
			name: "missing from",
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is:dest",
				},
				Tag: &imageapi.TagReference{
					Name: "dest",
				},
				Image: imageapi.Image{
					ObjectMeta: kapi.ObjectMeta{
						Name:        imagetest.MiscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", "test", "dest", imagetest.MiscImageDigest),
					DockerImageManifest:  imagetest.MiscImage,
				},
			},
			expectedImages: 0,
		},

		{
			name: "update existing tag",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "havingtag",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/havingtag@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
								},
							},
						},
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
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
			expectedImages: 1,
		},

		{
			name: "add a new tag with with 2 image streams",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "destis",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "other",
						Name:      "is2",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.BaseImageWith2LayersDigest),
										Image:                imagetest.BaseImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
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
			expectedImages: 1,
		},

		{
			name: "tag an image already present using image stream image",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "destis",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
						},
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "destis:latest",
				},
				Tag: &imageapi.TagReference{
					Name: "latest",
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamImage",
						Namespace: "shared",
						Name:      "is@" + imagetest.BaseImageWith1LayerDigest,
					},
				},
			},
			expectedImages: 0,
		},

		{
			name: "tag an image already present using image stream tag",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "destis",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
						},
					},
				},
			},
			ist: imageapi.ImageStreamTag{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "another:latest",
				},
				Tag: &imageapi.TagReference{
					Name: "latest",
					// shared is has name of baseImageWith1Layer at the first place in event list
					From: &kapi.ObjectReference{
						Kind:      "ImageStreamTag",
						Namespace: "shared",
						Name:      "is:latest",
					},
				},
			},
			expectedImages: 0,
		},
	} {

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, "test"))
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, tc.iss...))
		fakeClient.AddReactor("list", "imagestreams", imagetest.GetFakeImageStreamListHandler(t, tc.iss...))
		fakeClient.AddReactor("get", "imagestreamimages", imagetest.GetFakeImageStreamImageGetHandler(t, "test", tc.iss...))

		evaluator := NewImageStreamTagEvaluator(fakeClient)

		usage := evaluator.Usage(&tc.ist)

		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
		}

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
