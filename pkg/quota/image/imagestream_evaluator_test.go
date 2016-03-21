package image

import (
	"fmt"
	"strings"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	ktestclient "k8s.io/kubernetes/pkg/client/unversioned/testclient"
	kquota "k8s.io/kubernetes/pkg/quota"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/client/testclient"
	imageapi "github.com/openshift/origin/pkg/image/api"
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
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
									Image:                baseImageWith1LayerDigest,
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
								Name:      fmt.Sprintf("is@%s", miscImageDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
									Image:                baseImageWith1LayerDigest,
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
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith1LayerDigest),
									Image:                baseImageWith1LayerDigest,
								},
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith2LayersDigest),
									Image:                baseImageWith2LayersDigest,
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
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith2LayersDigest),
									Image:                baseImageWith2LayersDigest,
								},
							},
						},
						"bar": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", childImageWith3LayersDigest),
									Image:                childImageWith3LayersDigest,
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
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/noshared@%s", childImageWith2LayersDigest),
									Image:                childImageWith2LayersDigest,
								},
							},
						},
						"foo": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/noshared@%s", childImageWith2LayersDigest),
									Image:                childImageWith2LayersDigest,
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
		fakeClient.AddReactor("get", "imagestreams", getFakeImageStreamGetHandler(t, tc.is))
		fakeClient.AddReactor("get", "imagestreamimages", getFakeImageStreamImageGetHandler(t, tc.is))

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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
									},
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith2LayersDigest),
										Image:                baseImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", baseImageWith1LayerDigest),
										Image:                baseImageWith1LayerDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", baseImageWith2LayersDigest),
										Image:                baseImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", childImageWith2LayersDigest),
										Image:                childImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", miscImageDigest),
										Image:                miscImageDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is1@%s", childImageWith2LayersDigest),
										Image:                childImageWith2LayersDigest,
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
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", miscImageDigest),
										Image:                miscImageDigest,
									},
								},
							},
							"foo": {
								Items: []imageapi.TagEvent{
									{
										DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is2@%s", childImageWith2LayersDigest),
										Image:                childImageWith2LayersDigest,
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
		fakeClient.AddReactor("list", "imagestreams", getFakeImageStreamListHandler(t, tc.iss...))
		fakeClient.AddReactor("get", "imagestreamimages", getFakeImageStreamImageGetHandler(t, tc.iss...))

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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
							Name:      fmt.Sprintf("is@%s", miscImageDigest),
						},
					},
				},
			},
			oldStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
							Name:      fmt.Sprintf("is@%s", miscImageDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
							},
						},
					},
					"foo": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", childImageWith2LayersDigest),
								Image:                childImageWith2LayersDigest,
							},
						},
					},
					"bar": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith2LayersDigest),
								Image:                baseImageWith2LayersDigest,
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", miscImageDigest),
								Image:                miscImageDigest,
							},
						},
					},
					"bar": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", baseImageWith2LayersDigest),
								Image:                baseImageWith2LayersDigest,
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", childImageWith3LayersDigest),
								Image:                childImageWith3LayersDigest,
							},
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/sharedlayer@%s", miscImageDigest),
								Image:                miscImageDigest,
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
							Name:      fmt.Sprintf("is@%s", baseImageWith2LayersDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith1LayerDigest),
								Image:                baseImageWith1LayerDigest,
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
							Name:      fmt.Sprintf("is@%s", childImageWith2LayersDigest),
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
							Name:      fmt.Sprintf("is@%s", childImageWith2LayersDigest),
						},
					},
				},
			},
			newStatus: &imageapi.ImageStreamStatus{
				Tags: map[string]imageapi.TagEventList{
					"latest": {
						Items: []imageapi.TagEvent{
							{
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", baseImageWith2LayersDigest),
								Image:                childImageWith2LayersDigest,
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
							Name:      fmt.Sprintf("is@%s", miscImageDigest),
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
							Name: fmt.Sprintf("common@%s", miscImageDigest),
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
								DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/common@%s", miscImageDigest),
								Image:                miscImageDigest,
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
		fakeClient.AddReactor("get", "imagestreams", getFakeImageStreamGetHandler(t, iss...))
		fakeClient.AddReactor("list", "imagestreams", getFakeImageStreamListHandler(t, iss...))
		fakeClient.AddReactor("get", "images", getFakeImageGetHandler(t, "test"))

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

func getFakeImageStreamListHandler(t *testing.T, iss ...imageapi.ImageStream) ktestclient.ReactionFunc {
	sharedISs := []imageapi.ImageStream{*getSharedImageStream("shared", "is")}
	allISs := append(sharedISs, iss...)

	return func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case ktestclient.ListAction:
			res := &imageapi.ImageStreamList{
				Items: []imageapi.ImageStream{},
			}
			for _, is := range allISs {
				if is.Namespace == a.GetNamespace() {
					res.Items = append(res.Items, is)
				}
			}

			t.Logf("imagestream list handler: returning %d image streams from namespace %s", len(res.Items), a.GetNamespace())

			return true, res, nil
		}
		return false, nil, nil
	}
}

func getFakeImageStreamGetHandler(t *testing.T, iss ...imageapi.ImageStream) ktestclient.ReactionFunc {
	sharedISs := []imageapi.ImageStream{*getSharedImageStream("shared", "is")}
	allISs := append(sharedISs, iss...)

	return func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case ktestclient.GetAction:
			for _, is := range allISs {
				if is.Namespace == a.GetNamespace() && a.GetName() == is.Name {
					t.Logf("imagestream get handler: returning image stream %s/%s", is.Namespace, is.Name)
					return true, &is, nil
				}
			}

			err := fmt.Errorf("image stream %s/%s not found", a.GetNamespace(), a.GetName())
			t.Errorf(err.Error())
			return true, nil, err
		}
		return false, nil, nil
	}
}

func getSharedImageStream(namespace, name string) *imageapi.ImageStream {
	tevList := imageapi.TagEventList{}
	for _, imgName := range []string{
		baseImageWith1LayerDigest,
		baseImageWith2LayersDigest,
		childImageWith2LayersDigest,
		childImageWith3LayersDigest,
		miscImageDigest,
	} {
		tevList.Items = append(tevList.Items,
			imageapi.TagEvent{
				DockerImageReference: fmt.Sprintf("172.30.12.34:5000/test/is@%s", imgName),
				Image:                imgName,
			})
	}

	sharedIS := imageapi.ImageStream{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Status: imageapi.ImageStreamStatus{
			Tags: map[string]imageapi.TagEventList{
				"latest": tevList,
			},
		},
	}

	return &sharedIS
}

func getFakeImageStreamImageGetHandler(t *testing.T, iss ...imageapi.ImageStream) ktestclient.ReactionFunc {
	sharedIS := getSharedImageStream("shared", "is")

	return func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case ktestclient.GetAction:
			imageStreams := append([]imageapi.ImageStream{*sharedIS}, iss...)
			for _, is := range imageStreams {
				if (a.GetNamespace() != is.Namespace || !strings.HasPrefix(a.GetName(), is.Name+"@")) && (is.Namespace != "shared" || a.GetName() != "shared") {
					continue
				}
				nameParts := strings.SplitN(a.GetName(), "@", 2)
				name := nameParts[1]

				res := &imageapi.ImageStreamImage{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: a.GetNamespace(),
						Name:      a.GetName(),
					},
					Image: imageapi.Image{
						ObjectMeta: kapi.ObjectMeta{
							Name:        name,
							Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
						},
						DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", a.GetNamespace(), a.GetName()),
					},
				}

				switch name {
				case baseImageWith1LayerDigest:
					res.Image.DockerImageManifest = baseImageWith1Layer
				case baseImageWith2LayersDigest:
					res.Image.DockerImageManifest = baseImageWith2Layers
				case childImageWith2LayersDigest:
					res.Image.DockerImageManifest = childImageWith2Layers
				case childImageWith3LayersDigest:
					res.Image.DockerImageManifest = childImageWith3Layers
				case miscImageDigest:
					res.Image.DockerImageManifest = miscImage
				default:
					err := fmt.Errorf("image %q not found", name)
					t.Error(err.Error())
					return true, nil, err
				}

				t.Logf("imagestreamimage get handler: returning %q", res.Name)
				return true, res, nil
			}

			err := fmt.Errorf("imagestreamimage %q not found", a.GetName())
			t.Error(err.Error())
			return true, nil, err
		}
		return false, nil, nil
	}
}

func getFakeImageGetHandler(t *testing.T, namespace string) ktestclient.ReactionFunc {
	return func(action ktestclient.Action) (handled bool, ret runtime.Object, err error) {
		switch a := action.(type) {
		case ktestclient.GetAction:
			name := a.GetName()

			res := imageapi.Image{
				ObjectMeta: kapi.ObjectMeta{
					Name:        name,
					Annotations: map[string]string{imageapi.ManagedByOpenShiftAnnotation: "true"},
				},
				DockerImageReference: fmt.Sprintf("registry.example.org/%s/%s", namespace, a.GetName()),
			}

			switch name {
			case baseImageWith1LayerDigest:
				res.DockerImageManifest = baseImageWith1Layer
			case baseImageWith2LayersDigest:
				res.DockerImageManifest = baseImageWith2Layers
			case childImageWith2LayersDigest:
				res.DockerImageManifest = childImageWith2Layers
			case childImageWith3LayersDigest:
				res.DockerImageManifest = childImageWith3Layers
			case miscImageDigest:
				res.DockerImageManifest = miscImage
			default:
				err := fmt.Errorf("image %q not found", name)
				t.Error(err.Error())
				return true, nil, err
			}

			t.Logf("images get handler: returning %q", res.Name)
			return true, &res, nil
		}
		return false, nil, nil
	}
}

// 1 data layer of 128 B
const baseImageWith1LayerDigest = `sha256:c5207ce0f38da269ad2e58f143b5ea4b314c75ce1121384369f0db9015e10e82`
const baseImageWith1Layer = `{
   "schemaVersion": 1,
   "name": "miminar/baseImageWith1Layer",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
		  "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:d4994ff5bda31913c54af389d68d27418b294cde415cb41282b513900bd11f1e\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"container\":\"99664df33257d325a5d3c082e72a5b6bf86adf1d4e75af6c5a5c4cdaab1fac58\",\"container_config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"],\"Image\":\"sha256:d4994ff5bda31913c54af389d68d27418b294cde415cb41282b513900bd11f1e\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"created\":\"2016-02-15T07:30:37.655693399Z\",\"docker_version\":\"1.10.0\",\"id\":\"3303329125f4954da646b116f6e4a7e40d03656d4802340d46aca8a473d9c3e4\",\"os\":\"linux\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// 2 data layers, the first is shared with baseImageWith1Layer, total size of 240 B
const baseImageWith2LayersDigest = "sha256:77371f61c054608a4bb1a96b99f9be69f0868340f5c924ecd8813172f7cf853d"
const baseImageWith2Layers = `{
   "schemaVersion": 1,
   "name": "miminar/baseImageWith2Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:e7900a2e6943680b384950859a0616089757cae4d8c6e98db9cfec6c41fe2834"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
          "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:356b1cbd1af67cfa316c7066895954a69865b972abe680942c123e8bfbbd7458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"container\":\"686b99d75c4a744420c9a6bf9d3ba2548e72462e4719c8202878315f48083b2c\",\"container_config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:23d2e6ff1c67ff4caee900c71d58df6e37bfb9defe46085018c4ba29c3d2de5a in /data2\"],\"Image\":\"sha256:356b1cbd1af67cfa316c7066895954a69865b972abe680942c123e8bfbbd7458\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":{}},\"created\":\"2016-02-15T07:31:50.390272025Z\",\"docker_version\":\"1.10.0\",\"id\":\"61c8a7f2be3a9b6fcd46f24da46eedfd37200b0d067d487595942b5b8bacbce7\",\"os\":\"linux\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"size\":112}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// based on baseImageWith1Layer, it adds a new data layer of 126 B
const childImageWith2LayersDigest = "sha256:a9f073fbf2c9835711acd09081d87f5b7129ac6269e0df834240000f48abecd4"
const childImageWith2Layers = `{
   "schemaVersion": 1,
   "name": "miminar/childImageWith2Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:766b6e9134dc2819fae9c5e67d39e14272948bc8967df9a119418cca84cab089"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
          "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:27bc5bf237c48c2b41b0636a3876960a9adb6c2ac9ff95ac879d56b1046ba5a1\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"container\":\"c2d2505e43f4fd479aa21d356270d0791633e838284d7010cba1f61992907c69\",\"container_config\":{\"Hostname\":\"d7b63ae1152b\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:859e4175fd5743f276905245e351272b425232cfd3b30a3fc6bff351da308996 in /data3\"],\"Image\":\"sha256:27bc5bf237c48c2b41b0636a3876960a9adb6c2ac9ff95ac879d56b1046ba5a1\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"created\":\"2016-02-15T07:33:17.59074814Z\",\"docker_version\":\"1.10.0\",\"id\":\"e6a8e2793d6cad7d503aa5a3b55fd2c19b3b190d480a175b21d5f7b50c86d27b\",\"os\":\"linux\",\"parent\":\"84dc393745ff2631760c4bdbf1168af188fcd4606c1400c6900487fdc75a9ed5\",\"size\":126}"
      },
      {
         "v1Compatibility": "{\"id\":\"84dc393745ff2631760c4bdbf1168af188fcd4606c1400c6900487fdc75a9ed5\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"created\":\"2016-02-15T07:33:17.454934648Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// based on baseImageWith2Layers, it adds a new data layer of 70 B
const childImageWith3LayersDigest = "sha256:2282a6d553353756fa43ba8672807d3fe81f8fdef54b0f6a360d64aaef2f243a"
const childImageWith3Layers = `{
   "schemaVersion": 1,
   "name": "miminar/childImageWith3Layers",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:77ef66f4abb43c5e17bcacdfe744f6959365f6244b66a6565470083fbdd15178"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:e7900a2e6943680b384950859a0616089757cae4d8c6e98db9cfec6c41fe2834"
      },
      {
         "blobSum": "sha256:2d099e04ef6c850542d8ab916df2e9417cc799d39b78f64440e51402f1261a36"
      },
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"architecture\":\"amd64\",\"author\":\"miminar@redhat.com\",\"config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":null,\"Image\":\"sha256:8b0241d44c66c1bcf48c66d0465ee6bf6ac2117e9936a9ec2337122e08d109ef\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"container\":\"61c9522f27b7052081b61b72d70dd71ce7050566812f050158e03954b493e446\",\"container_config\":{\"Hostname\":\"686b99d75c4a\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":[\"PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin\"],\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) COPY file:7781db9ed3a36b0607009b073a99802a9ad834bbb5e3bcbcf83a7d27146a1a5b in /data4\"],\"Image\":\"sha256:8b0241d44c66c1bcf48c66d0465ee6bf6ac2117e9936a9ec2337122e08d109ef\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":[],\"Labels\":{}},\"created\":\"2016-02-15T07:36:13.703778299Z\",\"docker_version\":\"1.10.0\",\"id\":\"8e7b1ec73ed1d21747991c2101d1db51e97c4f62931bbaa575aeba11286d6748\",\"os\":\"linux\",\"parent\":\"fbe31426cd0e8c5545ddc5c8318499682d52ff96118e36e49616ac3aee32c47c\",\"size\":70}"
      },
      {
         "v1Compatibility": "{\"id\":\"fbe31426cd0e8c5545ddc5c8318499682d52ff96118e36e49616ac3aee32c47c\",\"parent\":\"9b1154060650718a3850e625464addb217c1064f18dd693cf635dfcabdc9de50\",\"created\":\"2016-02-15T07:36:13.585345649Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      },
      {
         "v1Compatibility": "{\"id\":\"9b1154060650718a3850e625464addb217c1064f18dd693cf635dfcabdc9de50\",\"parent\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"created\":\"2016-02-15T07:31:50.390272025Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:23d2e6ff1c67ff4caee900c71d58df6e37bfb9defe46085018c4ba29c3d2de5a in /data2\"]},\"size\":112}"
      },
      {
         "v1Compatibility": "{\"id\":\"1620fdccc2424391c3422467cec611bc32767d5bfae5bd8a2fb53c795e2a3e86\",\"parent\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.655693399Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) COPY file:90583fd8c765e40f7f2070c55da446e138b019b0712dee898d8193b66b05d48d in /data1\"]},\"size\":128}"
      },
      {
         "v1Compatibility": "{\"id\":\"3690474eb5b4b26fdfbd89c6e159e8cc376ca76ef48032a30fa6aafd56337880\",\"created\":\"2016-02-15T07:30:37.531741167Z\",\"container_config\":{\"Cmd\":[\"/bin/sh -c #(nop) MAINTAINER miminar@redhat.com\"]},\"throwaway\":true}"
      }
   ]
}`

// another base image with unique data layer of 554 B
const miscImageDigest = "sha256:2643199e5ed5047eeed22da854748ed88b3a63ba0497601ba75852f7b92d4640"
const miscImage = `{
   "schemaVersion": 1,
   "name": "miminar/misc",
   "tag": "latest",
   "architecture": "amd64",
   "fsLayers": [
      {
         "blobSum": "sha256:a3ed95caeb02ffe68cdd9fd84406680ae93d633cb16422d00e8a7c22955b46d4"
      },
      {
         "blobSum": "sha256:eeee0535bf3cec7a24bff2c6e97481afa3d37e2cdeff277c57cb5cbdb2fa9e92"
      }
   ],
   "history": [
      {
         "v1Compatibility": "{\"id\":\"964092b7f3e54185d3f425880be0b022bfc9a706701390e0ceab527c84dea3e3\",\"parent\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"created\":\"2016-01-15T18:06:41.282540103Z\",\"container\":\"4e937d31f242d087cce0ec5b9fdbceaf1a13b40704e9147962cc80947e4ab86b\",\"container_config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) CMD [\\\"sh\\\"]\"],\"Image\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"sh\"],\"Image\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\"}"
      },
      {
         "v1Compatibility": "{\"id\":\"9e77fef7a1c9f989988c06620dabc4020c607885b959a2cbd7c2283c91da3e33\",\"created\":\"2016-01-15T18:06:40.707908287Z\",\"container\":\"aded96b43f48d94eb80642c210b89f119ab2a233c1c7c7055104fb052937f12c\",\"container_config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":[\"/bin/sh\",\"-c\",\"#(nop) ADD file:a62b361be92f978752150570261ddc6fc21b025e3a28418820a1f39b7db7498c in /\"],\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"docker_version\":\"1.8.3\",\"config\":{\"Hostname\":\"aded96b43f48\",\"Domainname\":\"\",\"User\":\"\",\"AttachStdin\":false,\"AttachStdout\":false,\"AttachStderr\":false,\"Tty\":false,\"OpenStdin\":false,\"StdinOnce\":false,\"Env\":null,\"Cmd\":null,\"Image\":\"\",\"Volumes\":null,\"WorkingDir\":\"\",\"Entrypoint\":null,\"OnBuild\":null,\"Labels\":null},\"architecture\":\"amd64\",\"os\":\"linux\",\"Size\":554}"
      }
   ]
}`
