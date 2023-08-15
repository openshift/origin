package knownimagechecker

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"github.com/openshift/origin/pkg/monitortestframework"

	"github.com/openshift/origin/test/extended/util/image"

	monitorserialization "github.com/openshift/origin/pkg/monitor/serialization"

	configv1client "github.com/openshift/client-go/config/clientset/versioned/typed/config/v1"
	imagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/origin/pkg/monitor/monitorapi"
	"github.com/openshift/origin/pkg/test/ginkgo/junitapi"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"
	e2e "k8s.io/kubernetes/test/e2e/framework"
)

const testName = "[sig-arch] Only known images used by tests"

type clusterImageValidator struct {
	adminKubeConfig *rest.Config
}

func NewEnsureValidImages() monitortestframework.MonitorTest {
	return &clusterImageValidator{}
}

func (w *clusterImageValidator) StartCollection(ctx context.Context, adminRESTConfig *rest.Config, recorder monitorapi.RecorderWriter) error {
	w.adminKubeConfig = adminRESTConfig
	return nil
}

func (w *clusterImageValidator) CollectData(ctx context.Context, storageDir string, beginning, end time.Time) (monitorapi.Intervals, []*junitapi.JUnitTestCase, error) {
	return nil, nil, nil
}

func (*clusterImageValidator) ConstructComputedIntervals(ctx context.Context, startingIntervals monitorapi.Intervals, recordedResources monitorapi.ResourcesMap, beginning, end time.Time) (monitorapi.Intervals, error) {
	return nil, nil
}

// EvaluateTestsFromConstructedIntervals checks whether the cluster pulled an image that is
// outside the allowed list of images. The list is defined as a set of static test case images, the
// local cluster registry, any repository referenced by the image streams in the cluster's 'openshift'
// namespace, or the location that input images are cloned from. Only namespaces prefixed with 'e2e-'
// are checked.
// any image not in the allowed prefixes is considered a failure, as the user
// may have added a new test image without calling the appropriate helpers
func (w *clusterImageValidator) EvaluateTestsFromConstructedIntervals(ctx context.Context, finalIntervals monitorapi.Intervals) ([]*junitapi.JUnitTestCase, error) {
	if w.adminKubeConfig == nil {
		return []*junitapi.JUnitTestCase{
			{
				Name:      testName,
				SystemOut: "missing kubeconfig",
				FailureOutput: &junitapi.FailureOutput{
					Output: "missing kubeconfig",
				},
			},
		}, nil

	}

	fromRepository := image.GetGlobalFromRepository()

	// static allowed images
	allowedImages := sets.NewString("image/webserver:404")
	allowedPrefixes := sets.NewString(
		"image-registry.openshift-image-registry.svc",
		"gcr.io/k8s-authenticated-test/",
		"gcr.io/authenticated-image-pulling/",
		"invalid.com/",

		// installed alongside OLM and managed externally
		"registry.redhat.io/redhat/community-operator-index",
		"registry.redhat.io/redhat/certified-operator-index",
		"registry.redhat.io/redhat/redhat-marketplace-index",
		"registry.redhat.io/redhat/redhat-operator-index",

		// used by OLM tests
		"registry.redhat.io/amq7/amq-streams-rhel7-operator",
		"registry.redhat.io/amq7/amqstreams-rhel7-operator-metadata",

		// used to test pull secrets against an authenticated registry
		// TODO: will not work for a disconnected test environment and should be emulated by launching
		//   an authenticated registry in a pod on cluster
		"registry.redhat.io/ubi8/nodejs-14:latest",
	)
	if len(fromRepository) > 0 {
		allowedPrefixes.Insert(fromRepository)
	}

	imageStreamPrefixes, err := imagePrefixesFromNamespaceImageStreams("openshift")
	if err != nil {
		klog.Errorf("Unable to identify image prefixes from the openshift namespace: %v", err)
	}
	allowedPrefixes.Insert(imageStreamPrefixes.UnsortedList()...)

	releaseImage, err := getReleaseImage(w.adminKubeConfig)
	if err != nil {
		klog.Errorf("failed to get release image: %v", err)
	} else {
		allowedPrefixes.Insert(releaseImage)
	}

	allowedPrefixesSlice := allowedPrefixes.List()

	// these are the possible format we need to work with:
	// 1. reason/Pulled Container image "quay.io/openshift/community-e2e-images:e2e-7-k8s-gcr-io-e2e-test-images-busybox-1-29-2-cqcP1Tnbm-JjJyUA" already present on machine
	// 2. reason/Pulled image/quay.io/openshift/community-e2e-images:e2e-7-k8s-gcr-io-e2e-test-images-busybox-1-29-2-cqcP1Tnbm-JjJyUA
	// the regexp tries to match either image/(group) or image "(group)",
	// where (group) is constructed from three subgroups divided with /
	// where each has one or more characters from these:
	// \w (word characters - [0-9A-Za-z_]), -, :, . (needs escaping)
	imageRe, err := regexp.Compile(`image/([\w-:\.]+/[\w-:\.]+/[\w-:\.]+)$|image "([\w-:\.]+/[\w-:\.]+/[\w-:\.]+)"`)
	if err != nil {
		klog.Errorf("failed to compile regexp for image parsing")
	}

	pulls := make(map[string]sets.String)

	for _, event := range finalIntervals {
		// only messages that include a Pulled reason
		if !strings.Contains(" "+event.Message, " reason/Pulled ") {
			continue
		}
		// only look at pull events from an e2e-* namespace
		if !strings.Contains(" "+event.Locator, " ns/e2e-") {
			continue
		}

		images := imageRe.FindStringSubmatch(event.Message)
		// the images will contain full match and two group matches, see above
		// for the regexp definition, so we skip the first in the below for-loop
		if len(images) < 3 {
			continue
		}
		image := ""
		for i := 1; i < len(images); i++ {
			image = images[i]
			// the match will be either 2nd or 3rd element in the list
			if image != "" {
				break
			}
		}
		if hasAnyStringPrefix(image, allowedPrefixesSlice) || allowedImages.Has(image) {
			continue
		}
		byImage, ok := pulls[image]
		if !ok {
			byImage = sets.NewString()
			pulls[image] = byImage
			fmt.Printf("[sig-arch] unknown image: %s (%v)\n", image, event.Message)
		}
		byImage.Insert(event.Locator)
	}

	if len(pulls) > 0 {
		images := make([]string, 0, len(pulls))
		for image := range pulls {
			images = append(images, image)
		}
		sort.Strings(images)
		buf := &bytes.Buffer{}
		for _, image := range images {
			fmt.Fprintf(buf, "%s from pods:\n", image)
			for _, locator := range pulls[image].List() {
				fmt.Fprintf(buf, "  %s\n", locator)
			}
		}
		return []*junitapi.JUnitTestCase{
			{
				Name:      testName,
				SystemOut: buf.String(),
				FailureOutput: &junitapi.FailureOutput{
					Output: fmt.Sprintf("Cluster accessed images that were not mirrored to the testing repository or already part of the cluster, see test/extended/util/image/README.md in the openshift/origin repo:\n\n%s", buf.String()),
				},
			},
		}, nil
	}

	// if the test passed, indicate that too.
	return []*junitapi.JUnitTestCase{
		{
			Name: testName,
		},
	}, nil
}

func (*clusterImageValidator) WriteContentToStorage(ctx context.Context, storageDir, timeSuffix string, finalIntervals monitorapi.Intervals, finalResourceState monitorapi.ResourcesMap) error {
	return monitorserialization.EventsToFile(filepath.Join(storageDir, fmt.Sprintf("e2e-events%s.json", timeSuffix)), finalIntervals)
}

func (*clusterImageValidator) Cleanup(ctx context.Context) error {
	// TODO wire up the start to a context we can kill here
	return nil
}

func hasAnyStringPrefix(s string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(s, prefix) {
			return true
		}
	}
	return false
}

// imagePrefixesFromNamespaceImageStreams identifies all image repositories referenced by
// image streams in the provided namespace and returns them as a set (for both tags and
// digests). This set of prefixes can be used to verify that image references are coming
// from a location the cluster knows about.
func imagePrefixesFromNamespaceImageStreams(ns string) (sets.String, error) {
	clientConfig, err := e2e.LoadConfig(true)
	if err != nil {
		return nil, err
	}
	client, err := imagev1.NewForConfig(clientConfig)
	if err != nil {
		return nil, err
	}
	streams, err := client.ImageStreams(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	allowedPrefixes := sets.NewString()
	for _, stream := range streams.Items {
		for _, tag := range stream.Spec.Tags {
			if tag.From == nil || tag.From.Kind != "DockerImage" {
				continue
			}
			ref, err := reference.Parse(tag.From.Name)
			if err != nil {
				continue
			}
			repo := ref.AsRepository().Exact()
			allowedPrefixes.Insert(repo + ":")
			allowedPrefixes.Insert(repo + "@")
		}
		for _, tag := range stream.Status.Tags {
			for _, event := range tag.Items {
				if len(event.DockerImageReference) == 0 {
					continue
				}
				ref, err := reference.Parse(event.DockerImageReference)
				if err != nil {
					continue
				}
				repo := ref.AsRepository().Exact()
				allowedPrefixes.Insert(repo + ":")
				allowedPrefixes.Insert(repo + "@")
			}
		}
	}
	return allowedPrefixes, nil
}

// getReleaseImage does exactly that. We need to add it as exception, as there are some oauth tests that use it to find the
// oauth server image when the ControlPlaneToplogy is external, where there is no oauth server deployed inside the cluster that
// could be used: https://github.com/openshift/origin/blob/176aeb92845af9eb50b1d0fe8e98a78dee29215e/test/extended/util/oauthserver/oauthserver.go#L489-L532
func getReleaseImage(cfg *rest.Config) (string, error) {
	client, err := configv1client.NewForConfig(cfg)
	if err != nil {
		return "", fmt.Errorf("failed to construct configv1client: %w", err)
	}
	cv, err := client.ClusterVersions().Get(context.Background(), "version", metav1.GetOptions{})
	if err != nil {
		return "", fmt.Errorf("failed to get clusterversion: %w", err)
	}
	return cv.Status.Desired.Image, nil
}
