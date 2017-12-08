package admission

import (
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	quotautil "github.com/openshift/origin/pkg/quota/util"
)

func TestGetMaxLimits(t *testing.T) {
	for _, tc := range []struct {
		name           string
		lrs            []kapi.LimitRange
		expectedLimits kapi.ResourceList
	}{
		{
			name: "no limit range",
		},

		{
			name: "unrelevant limit range",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: kapi.LimitTypePod,
								Max:  kapi.ResourceList{kapi.ResourceCPU: resource.MustParse("200m")},
							},
						},
					},
				},
			},
		},

		{
			name: "max image stream images",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max:  kapi.ResourceList{imageapi.ResourceImageStreamImages: resource.MustParse("15")},
							},
						},
					},
				},
			},
			expectedLimits: kapi.ResourceList{imageapi.ResourceImageStreamImages: resource.MustParse("15")},
		},

		{
			name: "both limits",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("15"),
									imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
			},
			expectedLimits: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("15"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "both limits in two limit ranges",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("15"),
								},
							},
						},
					},
				},
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamTags: resource.MustParse("10"),
								},
							},
						},
					},
				},
			},
			expectedLimits: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("15"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "pick up the smaller",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("15"),
									imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("5"),
									imageapi.ResourceImageStreamTags:   resource.MustParse("20"),
								},
							},
						},
					},
				},
			},
			expectedLimits: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("5"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "pick up the smaller with unrelated resources",
			lrs: []kapi.LimitRange{
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								// min doesn't count
								Min: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("15"),
									imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamTags: resource.MustParse("10"),
									// ignored
									kapi.ResourceCPU: resource.MustParse("20"),
								},
							},
							{
								// wrong type
								Type: kapi.LimitTypeContainer,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamTags: resource.MustParse("5"),
								},
							},
						},
					},
				},
				{
					Spec: kapi.LimitRangeSpec{
						Limits: []kapi.LimitRangeItem{
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("25"),
								},
							},
							{
								Type: imageapi.LimitTypeImageStream,
								Max: kapi.ResourceList{
									imageapi.ResourceImageStreamImages: resource.MustParse("30"),
									imageapi.ResourceImageStreamTags:   resource.MustParse("20"),
								},
							},
						},
					},
				},
			},
			expectedLimits: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("25"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},
	} {
		var limits kapi.ResourceList
		for i := range tc.lrs {
			limits = getMaxLimits(&tc.lrs[i], limits)
		}
		if len(limits) != len(tc.expectedLimits) {
			t.Errorf("[%s] got unexpected number of limits (%d != %d)", tc.name, len(limits), len(tc.expectedLimits))
		}

		for r, v := range tc.expectedLimits {
			limit, exists := limits[r]
			if !exists {
				t.Errorf("[%s] expected resource %s is missing", tc.name, r)
				continue
			}
			if limit.Cmp(v) != 0 {
				t.Errorf("[%s] got unexpected value for resource %s (%s != %s)", tc.name, r, limit.String(), v.String())
			}
		}

		for r := range limits {
			if _, exists := tc.expectedLimits[r]; !exists {
				t.Errorf("[%s] got unexpected resource %s", tc.name, r)
			}
		}
	}
}

func TestVerifyLimits(t *testing.T) {
	for _, tc := range []struct {
		name              string
		maxUsage          kapi.ResourceList
		is                imageapi.ImageStream
		exceededResources []kapi.ResourceName
	}{
		{
			name: "no limits",
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
		},

		{
			name: "zero limits",
			maxUsage: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("0"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("0"),
			},
		},

		{
			name: "exceed images",
			maxUsage: kapi.ResourceList{
				imageapi.ResourceImageStreamImages: resource.MustParse("0"),
				imageapi.ResourceImageStreamTags:   resource.MustParse("0"),
			},
			is: imageapi.ImageStream{
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			exceededResources: []kapi.ResourceName{imageapi.ResourceImageStreamImages},
		},

		{
			name: "exceed tags",
			maxUsage: kapi.ResourceList{
				imageapi.ResourceImageStreamTags: resource.MustParse("0"),
			},
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			exceededResources: []kapi.ResourceName{imageapi.ResourceImageStreamTags},
		},

		{
			name: "exceed tags and images",
			maxUsage: kapi.ResourceList{
				imageapi.ResourceImageStreamTags:   resource.MustParse("1"),
				imageapi.ResourceImageStreamImages: resource.MustParse("0"),
			},
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
							},
						},
						"good": {
							Name: "good",
							From: &kapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
							},
						},
					},
				},
				Status: imageapi.ImageStreamStatus{
					Tags: map[string]imageapi.TagEventList{
						"latest": {
							Items: []imageapi.TagEvent{
								{
									DockerImageReference: imagetest.MakeDockerImageReference("test", "is", imagetest.BaseImageWith1LayerDigest),
									Image:                imagetest.BaseImageWith1LayerDigest,
								},
							},
						},
					},
				},
			},
			exceededResources: []kapi.ResourceName{
				imageapi.ResourceImageStreamTags,
				imageapi.ResourceImageStreamImages,
			},
		},
	} {
		limitRange := &kapi.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "limitrange",
			},
			Spec: kapi.LimitRangeSpec{
				Limits: []kapi.LimitRangeItem{
					{
						Type: imageapi.LimitTypeImageStream,
						Max:  tc.maxUsage,
					},
				},
			},
		}

		verifier := &limitVerifier{
			limiter: LimitRangesForNamespaceFunc(func(ns string) ([]*kapi.LimitRange, error) {
				return []*kapi.LimitRange{limitRange}, nil
			}),
		}

		err := verifier.VerifyLimits("test", &tc.is)
		if len(tc.exceededResources) > 0 && err == nil {
			t.Errorf("[%s] unexpected non-error while following resources should fail: %v", tc.name, tc.exceededResources)
			continue
		}
		if len(tc.exceededResources) == 0 && err != nil {
			t.Errorf("[%s] unexpected error: %v", tc.name, err)
			continue
		}
		for _, r := range tc.exceededResources {
			if !strings.Contains(err.Error(), string(r)) {
				t.Errorf("[%s] expected resource %q not found in error message: %v", tc.name, r, err)
			}
			if !quotautil.IsErrorQuotaExceeded(err) {
				t.Errorf("[%s] error %q not matched by IsErrorQuotaExceeded", tc.name, err)
			}
		}
	}
}
