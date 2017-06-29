package image

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	kapi "k8s.io/kubernetes/pkg/api"
	kquota "k8s.io/kubernetes/pkg/quota"

	imagetest "github.com/openshift/origin/pkg/image/admission/testutil"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	imageinformer "github.com/openshift/origin/pkg/image/generated/informers/internalversion"
	imageinternal "github.com/openshift/origin/pkg/image/generated/internalclientset/fake"
)

func TestImageStreamEvaluatorUsageStats(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imageapi.ImageStream
		namespace       string
		expectedISCount int64
	}{
		{
			name:            "no image stream",
			iss:             []imageapi.ImageStream{},
			namespace:       "test",
			expectedISCount: 0,
		},

		{
			name: "one image stream",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "onetag",
					},
				},
			},
			namespace:       "test",
			expectedISCount: 1,
		},

		{
			name: "two image streams",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is2",
					},
				},
			},
			namespace:       "test",
			expectedISCount: 2,
		},

		{
			name: "two image streams in different namespaces",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is1",
					},
				},
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "other",
						Name:      "is2",
					},
				},
			},
			namespace:       "test",
			expectedISCount: 1,
		},
	} {
		imageInformers := imageinformer.NewSharedInformerFactory(imageinternal.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().InternalVersion().ImageStreams()
		for _, is := range tc.iss {
			isInformer.Informer().GetIndexer().Add(&is)
		}
		evaluator := NewImageStreamEvaluator(isInformer.Lister())

		stats, err := evaluator.UsageStats(
			kquota.UsageStatsOptions{
				Resources: []kapi.ResourceName{imageapi.ResourceImageStreams},
				Namespace: tc.namespace,
			},
		)
		if err != nil {
			t.Errorf("[%s]: could not get usage stats for namespace %q: %v", tc.name, tc.namespace, err)
			continue
		}

		expectedUsage := imagetest.ExpectedResourceListFor(tc.expectedISCount)
		expectedResources := kquota.ResourceNames(expectedUsage)
		if len(stats.Used) != len(expectedResources) {
			t.Errorf("[%s]: got unexpected number of computed resources: %d != %d", tc.name, len(stats.Used), len(expectedResources))
		}

		masked := kquota.Mask(stats.Used, expectedResources)
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

func TestImageStreamEvaluatorUsage(t *testing.T) {
	for _, tc := range []struct {
		name            string
		iss             []imageapi.ImageStream
		expectedISCount int64
	}{
		{
			name:            "new image stream",
			expectedISCount: 1,
		},

		{
			name: "image stream already exists",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "is",
					},
				},
			},
			expectedISCount: 1,
		},

		{
			name: "new image stream in non-empty project",
			iss: []imageapi.ImageStream{
				{
					ObjectMeta: metav1.ObjectMeta{
						Namespace: "test",
						Name:      "existing",
					},
				},
			},
			expectedISCount: 1,
		},
	} {
		newIS := &imageapi.ImageStream{
			ObjectMeta: metav1.ObjectMeta{
				Namespace: "test",
				Name:      "is",
			},
		}
		imageInformers := imageinformer.NewSharedInformerFactory(imageinternal.NewSimpleClientset(), 0)
		isInformer := imageInformers.Image().InternalVersion().ImageStreams()
		for _, is := range tc.iss {
			isInformer.Informer().GetIndexer().Add(&is)
		}
		evaluator := NewImageStreamEvaluator(isInformer.Lister())

		usage, err := evaluator.Usage(newIS)
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
