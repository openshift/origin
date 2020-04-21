package main

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"

	imagev1 "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	"github.com/openshift/library-go/pkg/image/reference"

	"github.com/openshift/origin/pkg/monitor"
	"github.com/openshift/origin/pkg/test/ginkgo"
	"github.com/openshift/origin/test/extended/util/image"

	e2e "k8s.io/kubernetes/test/e2e/framework"
	k8simage "k8s.io/kubernetes/test/utils/image"
)

// OpenShift tests consume images from the cluster, from a number of vetted community locations,
// and from the upstream Kubernetes test suite which may reference images produced by a variety
// of build systems. To better organize and consolidate these images, the utility code within
// Kubernetes and OpenShift that is consulted to find image streams is made remappable - so that
// all images used by the test code can be located in one place. During normal operation the test
// images are read from the mirror, and administrators can choose to copy those images into
// restricted environment image registries and then run the tests against that subset. This also
// allows us to control the process whereby new images are introduced and review those in one spot.
//
// Test code utilizes helpers to get image strings throughout the code base, or is expected to use
// one or more of the images every OpenShift distribution includes in the 'openshift' namespace.
//
// See test/extended/util/image/README.md for a description of the process of adding a new image.

// defaultTestImageMirrorLocation is where all Kube test inputs are sourced.
const defaultTestImageMirrorLocation = "quay.io/openshift/community-e2e-images"

// createImageMirrorForInternalImages returns a list of 'oc image mirror' mappings from source to
// target or returns an error. If mirrored is true the images are assumed to have already been copied
// from their upstream location into our official mirror, in the REPO:TAG format where TAG is a hash
// of the original internal name and the index of the image in the array. Otherwise the mappings will
// be set to mirror the location as defined in the test code into our official mirror, where the target
// TAG is the hash described above.
func createImageMirrorForInternalImages(prefix string, ref reference.DockerImageReference, mirrored bool) ([]string, error) {
	source := ref.Exact()

	defaults := k8simage.GetOriginalImageConfigs()
	updated := k8simage.GetMappedImageConfigs(defaults, ref.Exact())

	openshiftDefaults := image.OriginalImages()
	openshiftUpdated := image.GetMappedImages(openshiftDefaults, defaultTestImageMirrorLocation)

	// if we've mirrored, then the source is going to be our repo, not upstream's
	if mirrored {
		baseRef, err := reference.Parse(defaultTestImageMirrorLocation)
		if err != nil {
			return nil, fmt.Errorf("invalid default mirror location: %v", err)
		}

		// calculate the mapping of upstream images by setting defaults to baseRef
		covered := sets.NewString()
		for i, config := range updated {
			defaultConfig := defaults[i]
			pullSpec := config.GetE2EImage()
			if pullSpec == defaultConfig.GetE2EImage() {
				continue
			}
			if covered.Has(pullSpec) {
				continue
			}
			covered.Insert(pullSpec)
			e2eRef, err := reference.Parse(pullSpec)
			if err != nil {
				return nil, fmt.Errorf("invalid test image: %s: %v", pullSpec, err)
			}
			if len(e2eRef.Tag) == 0 {
				return nil, fmt.Errorf("invalid test image: %s: no tag", pullSpec)
			}
			config.SetRegistry(baseRef.Registry)
			config.SetName(baseRef.RepositoryName())
			config.SetVersion(e2eRef.Tag)
			defaults[i] = config
		}

		// calculate the mapping for openshift images by populating openshiftUpdated
		openshiftUpdated = make(map[string]string)
		sourceMappings := image.GetMappedImages(openshiftDefaults, defaultTestImageMirrorLocation)
		targetMappings := image.GetMappedImages(openshiftDefaults, source)

		for from, to := range targetMappings {
			if from == to {
				continue
			}
			if covered.Has(to) {
				continue
			}
			covered.Insert(to)
			from := sourceMappings[from]
			openshiftUpdated[from] = to
		}
	}

	covered := sets.NewString()
	var lines []string
	for i := range updated {
		a, b := defaults[i], updated[i]
		from, to := a.GetE2EImage(), b.GetE2EImage()
		if from == to {
			continue
		}
		if covered.Has(from) {
			continue
		}
		covered.Insert(from)
		lines = append(lines, fmt.Sprintf("%s %s%s", from, prefix, to))
	}

	for from, to := range openshiftUpdated {
		if from == to {
			continue
		}
		if covered.Has(from) {
			continue
		}
		covered.Insert(from)
		lines = append(lines, fmt.Sprintf("%s %s%s", from, prefix, to))
	}

	sort.Strings(lines)
	return lines, nil
}

func verifyImages() error {
	if len(os.Getenv("KUBE_TEST_REPO")) > 0 {
		return fmt.Errorf("KUBE_TEST_REPO may not be specified when this command is run")
	}
	return verifyImagesWithoutEnv()
}

func verifyImagesWithoutEnv() error {
	defaults := k8simage.GetOriginalImageConfigs()

	for originalPullSpec, index := range image.OriginalImages() {
		if index == -1 {
			continue
		}
		existing, ok := defaults[index]
		if !ok {
			return fmt.Errorf("image %q not found in upstream images, must be moved to test/extended/util/image", originalPullSpec)
		}
		if existing.GetE2EImage() != originalPullSpec {
			return fmt.Errorf("image %q defines index %d but is defined upstream as %q, must be fixed in test/extended/util/image", originalPullSpec, index, existing.GetE2EImage())
		}
		mirror := image.LocationFor(originalPullSpec)
		upstreamMirror := k8simage.GetE2EImage(index)
		if mirror != upstreamMirror {
			return fmt.Errorf("image %q defines index %d and mirror %q but is mirrored upstream as %q, must be fixed in test/extended/util/image", originalPullSpec, index, mirror, upstreamMirror)
		}
	}

	return nil
}

// pulledInvalidImages returns a function that checks whether the cluster pulled an image that is
// outside the allowed list of images. The list is defined as a set of static test case images, the
// local cluster registry, any repository referenced by the image streams in the cluster's 'openshift'
// namespace, or the location that input images are cloned from.
func pulledInvalidImages(fromRepository string) func(events monitor.EventIntervals) ([]*ginkgo.JUnitTestCase, bool) {
	// static allowed images
	allowedImages := sets.NewString("image/webserver:404")
	allowedPrefixes := sets.NewString(
		"image-registry.openshift-image-registry.svc",
		"gcr.io/k8s-authenticated-test/",
		"gcr.io/authenticated-image-pulling/",
		"invalid.com/",

		// used by the CI infrastructure, eventually should be created as an image stream tag in
		// openshift so that it is automatically excluded
		"grafana/loki",
		"grafana/promtail",
		// this is used by an operator hub test and is not replaced today (in the future OLM should
		// use image streams to reference these and we can exclude those that match)
		"quay.io/helmoperators/cockroachdb",
	)
	if len(fromRepository) > 0 {
		allowedPrefixes.Insert(fromRepository)
	}

	imageStreamPrefixes, err := imagePrefixesFromNamespaceImageStreams("openshift")
	if err != nil {
		klog.Errorf("Unable to identify image prefixes from the openshift namespace: %v", err)
	}
	allowedPrefixes.Insert(imageStreamPrefixes.UnsortedList()...)

	// any image not in the allowed prefixes is considered a failure, as the user
	// may have added a new test image without calling the appropriate helpers
	return func(events monitor.EventIntervals) ([]*ginkgo.JUnitTestCase, bool) {
		allowedPrefixes := allowedPrefixes.List()

		passed := true
		var tests []*ginkgo.JUnitTestCase

		pulls := make(map[string]sets.String)
		for _, event := range events {
			if !strings.Contains(event.Message, " reason/Pulled ") {
				continue
			}
			parts := strings.Split(event.Message, " ")
			if len(parts) == 0 {
				continue
			}
			image := strings.TrimPrefix(parts[len(parts)-1], "image/")
			if hasAnyStringPrefix(image, allowedPrefixes) || allowedImages.Has(image) {
				continue
			}
			byImage, ok := pulls[image]
			if !ok {
				byImage = sets.NewString()
				pulls[image] = byImage
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
			tests = append(tests, &ginkgo.JUnitTestCase{
				Name:      "[sig-arch] Only known images used by tests",
				SystemOut: buf.String(),
				FailureOutput: &ginkgo.FailureOutput{
					Output: fmt.Sprintf("Cluster accessed images that were not mirrored to the testing repository or already part of the cluster, see test/extended/util/image/README.md in the openshift/origin repo:\n\n%s", buf.String()),
				},
			})
			passed = false
		}

		return tests, passed
	}
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
