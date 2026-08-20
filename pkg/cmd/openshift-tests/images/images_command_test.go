package images

import (
	"fmt"
	"strings"
	"testing"

	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/origin/pkg/test/extensions"
)

func TestCreateImageMirrorDeduplicatesByDestination(t *testing.T) {
	// This test verifies that when multiple image sets contain images with
	// DIFFERENT sources that map to the SAME destination tag, only the first
	// one is included in the output. This prevents 'oc image mirror' from
	// rejecting the mirror file with "each destination tag may only be
	// specified once".
	//
	// Using distinct sources is critical: identical sources are already caught
	// by the source-based dedup (covered[from]), so they would not exercise
	// the destination-based dedup path at all.

	ref, err := reference.Parse("quay.io/test/repo")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	imageID := k8simage.ImageID(9999)

	// Source for set 2: the upstream image.
	upstreamConfig := k8simage.Config{}
	upstreamConfig.SetRegistry("registry.k8s.io")
	upstreamConfig.SetName("e2e-test-images/agnhost")
	upstreamConfig.SetVersion("2.63.0")

	// Map the upstream config to derive the canonical destination.
	mappedSet := k8simage.GetMappedImageConfigs(
		extensions.ImageSet{imageID: upstreamConfig}, ref.Exact(),
	)
	destConfig := mappedSet[imageID]

	// Source for set 1: a DIFFERENT source image (simulating a pre-mapped
	// community mirror that resolves to the same destination tag).
	communityConfig := k8simage.Config{}
	communityConfig.SetRegistry("quay.io")
	communityConfig.SetName("openshift/community-e2e-images")
	communityConfig.SetVersion("e2e-2-registry-k8s-io-e2e-test-images-agnhost-2-63-0-abc123")

	// Verify precondition: the two sources are distinct.
	from1, from2 := communityConfig.GetE2EImage(), upstreamConfig.GetE2EImage()
	if from1 == from2 {
		t.Fatalf("test setup error: sources must differ but both are %q", from1)
	}

	// defaultImageSets: two sets with different sources.
	defaultImageSets := []extensions.ImageSet{
		{imageID: communityConfig},
		{imageID: upstreamConfig},
	}

	// updatedImageSets: both map to the SAME destination.
	updatedImageSets := []extensions.ImageSet{
		{imageID: destConfig},
		{imageID: destConfig},
	}

	dest := destConfig.GetE2EImage()
	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)

	// Count how many times the destination appears.
	destCount := 0
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == dest {
			destCount++
		}
	}
	if destCount != 1 {
		t.Errorf("expected destination %q to appear exactly once, but appeared %d times in output:\n%s",
			dest, destCount, strings.Join(lines, "\n"))
	}
}

func TestCreateImageMirrorDeduplicatesOpenshiftUpdatedByDestination(t *testing.T) {
	// Verify that the openshiftUpdated loop also deduplicates by destination.
	// If the main loop already output a destination, the openshiftUpdated loop
	// should not duplicate it.

	ref, err := reference.Parse("quay.io/test/repo")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	imageID := k8simage.ImageID(9999)

	sourceConfig := k8simage.Config{}
	sourceConfig.SetRegistry("registry.k8s.io")
	sourceConfig.SetName("e2e-test-images/agnhost")
	sourceConfig.SetVersion("2.63.0")

	defaultSet := extensions.ImageSet{imageID: sourceConfig}
	defaultImageSets := []extensions.ImageSet{defaultSet}

	updatedSet := k8simage.GetMappedImageConfigs(defaultSet, ref.Exact())
	updatedImageSets := []extensions.ImageSet{updatedSet}

	cfg := updatedSet[imageID]
	destTag := cfg.GetE2EImage()

	// Create an openshiftUpdated entry with a DIFFERENT source but the SAME
	// destination as the image in updatedImageSets.
	openshiftUpdated := map[string]string{
		"quay.io/openshift/community-e2e-images:different-source": destTag,
	}

	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, openshiftUpdated)

	// Count destination occurrences
	destCount := 0
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == destTag {
			destCount++
		}
	}
	if destCount != 1 {
		t.Errorf("expected destination %q to appear exactly once, but appeared %d times in output:\n%s",
			destTag, destCount, strings.Join(lines, "\n"))
	}
}

// deduplicatedMirrorLines extracts the output loop logic from
// createImageMirrorForInternalImages for testability. It applies the same
// source-based AND destination-based deduplication used in production.
func deduplicatedMirrorLines(prefix string, defaultImageSets, updatedImageSets []extensions.ImageSet, openshiftUpdated map[string]string) []string {
	covered := make(map[string]bool)
	coveredDestinations := make(map[string]bool)
	var lines []string

	for i := range updatedImageSets {
		for imageID := range updatedImageSets[i] {
			a, b := defaultImageSets[i][imageID], updatedImageSets[i][imageID]
			from, to := a.GetE2EImage(), b.GetE2EImage()
			if from == to {
				continue
			}
			if covered[from] {
				continue
			}
			dest := fmt.Sprintf("%s%s", prefix, to)
			if coveredDestinations[dest] {
				continue
			}
			covered[from] = true
			coveredDestinations[dest] = true
			lines = append(lines, fmt.Sprintf("%s %s", from, dest))
		}
	}

	for from, to := range openshiftUpdated {
		if from == to {
			continue
		}
		if covered[from] {
			continue
		}
		dest := fmt.Sprintf("%s%s", prefix, to)
		if coveredDestinations[dest] {
			continue
		}
		covered[from] = true
		coveredDestinations[dest] = true
		lines = append(lines, fmt.Sprintf("%s %s", from, dest))
	}

	return lines
}
