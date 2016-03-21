package image

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	kquota "k8s.io/kubernetes/pkg/quota"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
						Name:      "is@" + miscImageDigest,
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
						Name:      "is@" + miscImageDigest,
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
						Name:        miscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", "shared", "is", miscImageDigest),
					DockerImageManifest:  miscImageDigest,
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
						Name:        miscImageDigest,
						Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
					},
					DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", "test", "dest", miscImageDigest),
					DockerImageManifest:  miscImage,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/havingtag@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
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
						Name:      "is@" + childImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", miscImageDigest),
										Image:                miscImageDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", baseImageWith2LayersDigest),
										Image:                baseImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", miscImageDigest),
										Image:                miscImageDigest,
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
						Name:      "is@" + baseImageWith1LayerDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/destis@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", miscImageDigest),
										Image:                miscImageDigest,
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
		fakeClient.AddReactor("get", "images", getFakeImageGetHandler(t, "ns"))
		fakeClient.AddReactor("get", "imagestreams", getFakeImageStreamGetHandler(t, tc.iss...))
		fakeClient.AddReactor("list", "imagestreams", getFakeImageStreamListHandler(t, tc.iss...))
		fakeClient.AddReactor("get", "imagestreamimages", getFakeImageStreamImageGetHandler(t, tc.iss...))

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
