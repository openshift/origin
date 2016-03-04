package image

import (
	"fmt"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

func TestImageStreamMappingEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name             string
		iss              []imageapi.ImageStream
		imageName        string
		imageManifest    string
		imageAnnotations map[string]string
		destISNamespace  string
		destISName       string
		expectedUsage    kapi.ResourceList
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
			imageName:        miscImageDigest,
			imageManifest:    miscImage,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "is",
			expectedUsage: kapi.ResourceList{
				// increase usage for the whole misc image size
				imageapi.ResourceProjectImagesSize: resource.MustParse("554"),
				imageapi.ResourceImageStreamSize:   resource.MustParse("554"),
				imageapi.ResourceImageSize:         resource.MustParse("554"),
			},
		},

		{
			name:             "no image stream",
			imageName:        miscImageDigest,
			imageManifest:    miscImage,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "is",
		},

		{
			name: "missing image annotation",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
					Status: imageapi.ImageStreamStatus{},
				},
			},
			imageName:       miscImageDigest,
			imageManifest:   miscImage,
			destISNamespace: "test",
			destISName:      "is",
			expectedUsage: kapi.ResourceList{
				imageapi.ResourceProjectImagesSize: resource.MustParse("0"),
				imageapi.ResourceImageStreamSize:   resource.MustParse("0"),
				imageapi.ResourceImageSize:         resource.MustParse("0"),
			},
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
			imageName:        childImageWith2LayersDigest,
			imageManifest:    childImageWith2Layers,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "havingtag",
			expectedUsage: kapi.ResourceList{
				// count only the second data layer (first is already present in the image stream)
				imageapi.ResourceProjectImagesSize: resource.MustParse("126"),
				// compute whole registry size - original image is contained in the new one
				imageapi.ResourceImageStreamSize: resource.MustParse("254"),
				// compute whole image size
				imageapi.ResourceImageSize: resource.MustParse("254"),
			},
		},

		{
			name: "add a new tag with with 2 image streams ",
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
			imageName:        childImageWith3LayersDigest,
			imageManifest:    childImageWith3Layers,
			imageAnnotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
			destISNamespace:  "test",
			destISName:       "destis",
			expectedUsage: kapi.ResourceList{
				// count only the last 2 data layers of the image (first layer is already in the is)
				imageapi.ResourceProjectImagesSize: resource.MustParse("182"),
				// compute whole registry size
				imageapi.ResourceImageStreamSize: resource.MustParse("864"),
				// compute whole image size
				imageapi.ResourceImageSize: resource.MustParse("310"),
			},
		},
	} {

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("get", "imagestreams", func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
			switch a := action.(type) {
			case ktestclient.GetAction:
				for _, is := range tc.iss {
					if a.GetNamespace() != is.Namespace {
						continue
					}
					if a.GetName() != is.Name {
						continue
					}

					t.Logf("imagestream get handler: returning image stream %s/%s", is.Namespace, is.Name)
					return true, &is, nil
				}
				return true, nil, fmt.Errorf("image stream %s/%s not found", a.GetNamespace(), a.GetName())
			}

			return false, nil, nil
		})
		fakeClient.AddReactor("get", "imagestreamimages", getFakeImageStreamImageGetHandler(t, tc.destISNamespace, tc.iss...))

		evaluator := NewImageStreamMappingEvaluator(fakeClient)

		ism := &imageapi.ImageStreamMapping{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: tc.destISNamespace,
				Name:      tc.destISName,
			},
			Image: imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name:        tc.imageName,
					Annotations: tc.imageAnnotations,
				},
				DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s@%s", tc.destISNamespace, tc.destISName, tc.imageName),
				DockerImageManifest:  tc.imageManifest,
			},
		}

		usage := evaluator.Usage(ism)

		if len(usage) != len(tc.expectedUsage) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(usage), len(expectedResources))
		}

		expectedResourceNames := kquota.ResourceNames(tc.expectedUsage)
		masked := kquota.Mask(usage, expectedResourceNames)

		if len(masked) != len(tc.expectedUsage) {
			for k := range usage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: got unexpected resource %q from Usage() method", tc.name, k)
				}
			}

			for k := range tc.expectedUsage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: expected resource %q not computed", tc.name, k)
				}
			}
		}

		for rname, expectedValue := range tc.expectedUsage {
			if v, exists := masked[rname]; exists {
				if v.Cmp(expectedValue) != 0 {
					t.Errorf("[%s]: got unexpected usage for %q: %s != %s", tc.name, rname, v.String(), expectedValue.String())
				}
			}
		}
	}
}
