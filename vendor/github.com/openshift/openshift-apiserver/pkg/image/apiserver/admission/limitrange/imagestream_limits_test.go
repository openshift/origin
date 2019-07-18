package limitrange

import (
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	coreapi "k8s.io/kubernetes/pkg/apis/core"

	imagev1 "github.com/openshift/api/image/v1"
	"github.com/openshift/library-go/pkg/quota/quotautil"

	imageapi "github.com/openshift/openshift-apiserver/pkg/image/apis/image"
	imagetest "github.com/openshift/openshift-apiserver/pkg/image/apiserver/testutil"
)

func TestGetMaxLimits(t *testing.T) {
	for _, tc := range []struct {
		name           string
		lrs            []corev1.LimitRange
		expectedLimits corev1.ResourceList
	}{
		{
			name: "no limit range",
		},

		{
			name: "unrelevant limit range",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: corev1.LimitTypePod,
								Max:  corev1.ResourceList{corev1.ResourceCPU: resource.MustParse("200m")},
							},
						},
					},
				},
			},
		},

		{
			name: "max image stream images",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max:  corev1.ResourceList{imagev1.ResourceImageStreamImages: resource.MustParse("15")},
							},
						},
					},
				},
			},
			expectedLimits: corev1.ResourceList{imagev1.ResourceImageStreamImages: resource.MustParse("15")},
		},

		{
			name: "both limits",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("15"),
									imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
			},
			expectedLimits: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("15"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "both limits in two limit ranges",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("15"),
								},
							},
						},
					},
				},
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamTags: resource.MustParse("10"),
								},
							},
						},
					},
				},
			},
			expectedLimits: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("15"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "pick up the smaller",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("15"),
									imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("5"),
									imagev1.ResourceImageStreamTags:   resource.MustParse("20"),
								},
							},
						},
					},
				},
			},
			expectedLimits: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("5"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},

		{
			name: "pick up the smaller with unrelated resources",
			lrs: []corev1.LimitRange{
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								// min doesn't count
								Min: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("15"),
									imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
								},
							},
						},
					},
				},
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamTags: resource.MustParse("10"),
									// ignored
									corev1.ResourceCPU: resource.MustParse("20"),
								},
							},
							{
								// wrong type
								Type: corev1.LimitTypeContainer,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamTags: resource.MustParse("5"),
								},
							},
						},
					},
				},
				{
					Spec: corev1.LimitRangeSpec{
						Limits: []corev1.LimitRangeItem{
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("25"),
								},
							},
							{
								Type: imagev1.LimitTypeImageStream,
								Max: corev1.ResourceList{
									imagev1.ResourceImageStreamImages: resource.MustParse("30"),
									imagev1.ResourceImageStreamTags:   resource.MustParse("20"),
								},
							},
						},
					},
				},
			},
			expectedLimits: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("25"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("10"),
			},
		},
	} {
		var limits corev1.ResourceList
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
		maxUsage          corev1.ResourceList
		is                imageapi.ImageStream
		exceededResources []corev1.ResourceName
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
			maxUsage: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("0"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("0"),
			},
		},

		{
			name: "exceed images",
			maxUsage: corev1.ResourceList{
				imagev1.ResourceImageStreamImages: resource.MustParse("0"),
				imagev1.ResourceImageStreamTags:   resource.MustParse("0"),
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
			exceededResources: []corev1.ResourceName{imagev1.ResourceImageStreamImages},
		},

		{
			name: "exceed tags",
			maxUsage: corev1.ResourceList{
				imagev1.ResourceImageStreamTags: resource.MustParse("0"),
			},
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &coreapi.ObjectReference{
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
			exceededResources: []corev1.ResourceName{imagev1.ResourceImageStreamTags},
		},

		{
			name: "exceed tags and images",
			maxUsage: corev1.ResourceList{
				imagev1.ResourceImageStreamTags:   resource.MustParse("1"),
				imagev1.ResourceImageStreamImages: resource.MustParse("0"),
			},
			is: imageapi.ImageStream{
				Spec: imageapi.ImageStreamSpec{
					Tags: map[string]imageapi.TagReference{
						"new": {
							Name: "new",
							From: &coreapi.ObjectReference{
								Kind: "DockerImage",
								Name: imagetest.MakeDockerImageReference("test", "noshared", imagetest.ChildImageWith2LayersDigest),
							},
						},
						"good": {
							Name: "good",
							From: &coreapi.ObjectReference{
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
			exceededResources: []corev1.ResourceName{
				imagev1.ResourceImageStreamTags,
				imagev1.ResourceImageStreamImages,
			},
		},
	} {
		limitRange := &corev1.LimitRange{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "limitrange",
			},
			Spec: corev1.LimitRangeSpec{
				Limits: []corev1.LimitRangeItem{
					{
						Type: imagev1.LimitTypeImageStream,
						Max:  tc.maxUsage,
					},
				},
			},
		}

		verifier := &limitVerifier{
			limiter: LimitRangesForNamespaceFunc(func(ns string) ([]*corev1.LimitRange, error) {
				return []*corev1.LimitRange{limitRange}, nil
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
