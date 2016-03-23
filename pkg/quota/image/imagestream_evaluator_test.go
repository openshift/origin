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

var (
	expectedResources = []kapi.ResourceName{
		imageapi.ResourceImages,
	}
)

func TestImageStreamEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		is             imageapi.ImageStream
		expectedImages int64
	}{
		{
			"empty image stream",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "empty",
				},
				Status: imageapi.ImageStreamStatus{},
			},
			0,
		},

		{
			"image stream with one tag",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			1,
		},

		{
			"image stream with spec filled",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind:      "ImageStreamImage",
								Namespace: "shared",
								Name:      fmt.Sprintf("is@%s", imagetest.MiscImageDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			1, // spec isn't taken into account
		},

		{
			"image stream with two images under one tag",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "sharedlayer",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith2LayersDigest),
									Image:                imagetest.BaseImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			2,
		},

		{
			"image stream with two tags",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "sharedlayer",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith2LayersDigest),
									Image:                imagetest.BaseImageWith2LayersDigest,
								},
							},
						},
						"bar": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.ChildImageWith3LayersDigest),
									Image:                imagetest.ChildImageWith3LayersDigest,
								},
							},
						},
					},
				},
			},
			2,
		},

		{
			"image stream with the same image under different tag",
			imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "noshared",
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/noshared@%s", imagetest.ChildImageWith2LayersDigest),
									Image:                imagetest.ChildImageWith2LayersDigest,
								},
							},
						},
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/noshared@%s", imagetest.ChildImageWith2LayersDigest),
									Image:                imagetest.ChildImageWith2LayersDigest,
								},
							},
						},
					},
				},
			},
			1,
		},
	} {

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, tc.is))
		fakeClient.AddReactor("get", "imagestreamimages", imagetest.GetFakeImageStreamImageGetHandler(t, tc.is.Namespace, tc.is))

		evaluator := NewImageStreamEvaluator(fakeClient)

		is, err := evaluator.Get(tc.is.Namespace, tc.is.Name)
		if err != nil {
			t.Errorf("[%s]: could not get image stream %q: %v", tc.name, tc.is.Name, err)
			continue
		}
		usage := evaluator.Usage(is)

		if len(usage) != len(expectedResources) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(usage), len(expectedResources))
		}

		masked := kquota.Mask(usage, expectedResources)
		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
		}

		if len(masked) != len(expectedResources) {
			for k := range usage {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: got unexpected resource %q from Usage() method", tc.name, k)
				}
			}

			for _, k := range expectedResources {
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

func TestImageStreamEvaluatorUsageStats(t *testing.T) {
	for _, tc := range []struct {
		name           string
		iss            []imageapi.ImageStream
		namespace      string
		expectedImages int64
	}{
		{
			"no image stream",
			[]imageapi.ImageStream{},
			"test",
			0,
		},

		{
			"one image stream with one tag",
			[]imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "onetag",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
								},
							},
						},
					},
				},
			},
			"test",
			1,
		},

		{
			"image stream with two references under one tag",
			[]imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "sharedlayer",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith2LayersDigest),
										Image:                imagetest.BaseImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
			},
			"test",
			2,
		},

		{
			"two images in two image streams",
			[]imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is1",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", imagetest.BaseImageWith1LayerDigest),
										Image:                imagetest.BaseImageWith1LayerDigest,
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
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
			"test",
			2,
		},

		{
			"two image streams in different namespaces",
			[]imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is1",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", imagetest.ChildImageWith2LayersDigest),
										Image:                imagetest.ChildImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
						},
					},
				},
			},
			"test",
			1,
		},

		{
			"same image in two different image streams",
			[]imageapi.ImageStream{
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is1",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", imagetest.ChildImageWith2LayersDigest),
										Image:                imagetest.ChildImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
				{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: "test",
						Name:      "is2",
					},
					Status: imageapi.ImageStreamStatus{
						Tags: map[string]imageapi.TagEventList{
							"latest": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.MiscImageDigest),
										Image:                imagetest.MiscImageDigest,
									},
								},
							},
							"foo": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", imagetest.ChildImageWith2LayersDigest),
										Image:                imagetest.ChildImageWith2LayersDigest,
									},
								},
							},
						},
					},
				},
			},
			"test",
			2,
		},
	} {
		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("list", "imagestreams", imagetest.GetFakeImageStreamListHandler(t, tc.iss...))
		fakeClient.AddReactor("get", "imagestreamimages", imagetest.GetFakeImageStreamImageGetHandler(t, tc.namespace, tc.iss...))

		evaluator := NewImageStreamEvaluator(fakeClient)

		stats, err := evaluator.UsageStats(kquota.UsageStatsOptions{Namespace: tc.namespace})
		if err != nil {
			t.Errorf("[%s]: could not get usage stats for namespace %q: %v", tc.name, tc.namespace, err)
			continue
		}

		if len(stats.Used) != len(expectedResources) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(stats.Used), len(expectedResources))
		}

		masked := kquota.Mask(stats.Used, expectedResources)
		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
		}

		if len(masked) != len(expectedResources) {
			for k := range stats.Used {
				if _, exists := masked[k]; !exists {
					t.Errorf("[%s]: got unexpected resource %q from Usage() method", tc.name, k)
				}
			}

			for _, k := range expectedResources {
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

func TestImageStreamAdmissionEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name           string
		oldSpec        *imageapi.ImageStreamSpec
		oldStatus      *imageapi.ImageStreamStatus
		newSpec        *imageapi.ImageStreamSpec
		newStatus      *imageapi.ImageStreamStatus
		expectedImages int64
	}{
		{
			name:           "empty image stream",
			oldStatus:      nil,
			newStatus:      &imageapi.ImageStreamStatus{},
			expectedImages: 0,
		},

		{
			name:      "new image stream with one image",
			oldStatus: nil,
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			expectedImages: 1,
		},

		{
			name: "no change",
			oldSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"new": {
						Name: "new",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.MiscImageDigest),
						},
					},
				},
			},
			oldStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"new": {
						Name: "new",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.MiscImageDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			// misc image is already present in common is
			expectedImages: 1,
		},

		{
			name: "adding two new tags",
			oldStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
					"foo": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.ChildImageWith2LayersDigest),
								Image:                imagetest.ChildImageWith2LayersDigest,
							},
						},
					},
					"bar": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith2LayersDigest),
								Image:                imagetest.BaseImageWith2LayersDigest,
							},
						},
					},
				},
			},
			expectedImages: 3,
		},

		{
			name: "adding an item and deleting the other tag",
			oldStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"foo": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.MiscImageDigest),
								Image:                imagetest.MiscImageDigest,
							},
						},
					},
					"bar": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.BaseImageWith2LayersDigest),
								Image:                imagetest.BaseImageWith2LayersDigest,
							},
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"foo": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.ChildImageWith3LayersDigest),
								Image:                imagetest.ChildImageWith3LayersDigest,
							},
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", imagetest.MiscImageDigest),
								Image:                imagetest.MiscImageDigest,
							},
						},
					},
				},
			},
			// misc image is already present in common is
			expectedImages: 1,
		},

		{
			name: "adding a tag to spec",
			oldStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"new": {
						Name: "new",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.BaseImageWith2LayersDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith1LayerDigest),
								Image:                imagetest.BaseImageWith1LayerDigest,
							},
						},
					},
				},
			},
			expectedImages: 2,
		},

		{
			name: "adding a tag to status already present in spec",
			oldSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"latest": {
						Name: "new",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.ChildImageWith2LayersDigest),
						},
					},
				},
			},
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"latest": {
						Name: "new",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.ChildImageWith2LayersDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imagetest.BaseImageWith2LayersDigest),
								Image:                imagetest.ChildImageWith2LayersDigest,
							},
						},
					},
				},
			},
			expectedImages: 1,
		},

		{
			name: "refer to image in another namespace already present",
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"misc": {
						Name: "misc",
						From: &kapi.ObjectReference{
							Kind:      "ImageStreamImage",
							Namespace: "shared",
							Name:      fmt.Sprintf("is@%s", imagetest.MiscImageDigest),
						},
					},
				},
			},
			expectedImages: 0,
		},

		{
			name: "refer to imagestreamimage in the same namespace",
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"commonisi": {
						Name: "commonisi",
						From: &kapi.ObjectReference{
							Kind: "ImageStreamImage",
							Name: fmt.Sprintf("common@%s", imagetest.MiscImageDigest),
						},
					},
				},
			},
			expectedImages: 0,
		},

		{
			name: "refer to imagestreamtag in the same namespace",
			newSpec: &imageapi.ImageStreamSpec{
				Tags: map[string]imageapi.TagReference{
					"commonist": {
						Name: "commonist",
						From: &kapi.ObjectReference{
							Kind: "ImageStreamTag",
							Name: "common:misc",
						},
					},
				},
			},
			expectedImages: 0,
		},
	} {

		var newIS, oldIS *imageapi.ImageStream

		if tc.oldStatus != nil || tc.oldSpec != nil {
			oldIS = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
			}
			if tc.oldSpec != nil {
				oldIS.Spec = *tc.oldSpec
			}
			if tc.oldStatus != nil {
				oldIS.Status = *tc.oldStatus
			}
		}

		if tc.newStatus != nil || tc.newSpec != nil {
			newIS = &imageapi.ImageStream{
				ObjectMeta: kapi.ObjectMeta{
					Namespace: "test",
					Name:      "is",
				},
			}
			if tc.newSpec != nil {
				newIS.Spec = *tc.newSpec
			}
			if tc.newStatus != nil {
				newIS.Status = *tc.newStatus
			}
		}

		commonIS := imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: "test",
				Name:      "common",
			},
			Status: imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"misc": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/common@%s", imagetest.MiscImageDigest),
								Image:                imagetest.MiscImageDigest,
							},
						},
					},
				},
			},
		}
		iss := []imageapi.ImageStream{commonIS}
		if oldIS != nil {
			iss = append(iss, *oldIS)
		}

		fakeClient := &testclient.Fake{}
		fakeClient.AddReactor("get", "imagestreams", imagetest.GetFakeImageStreamGetHandler(t, iss...))
		fakeClient.AddReactor("list", "imagestreams", imagetest.GetFakeImageStreamListHandler(t, iss...))
		fakeClient.AddReactor("get", "images", imagetest.GetFakeImageGetHandler(t, "test"))

		evaluator := NewImageStreamAdmissionEvaluator(fakeClient)

		usage := evaluator.Usage(newIS)

		if len(usage) != len(expectedResources) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(usage), len(expectedResources))
		}

		expectedUsage := kapi.ResourceList{
			imageapi.ResourceImages: *resource.NewQuantity(tc.expectedImages, resource.DecimalSI),
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
