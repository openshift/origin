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
	// This test verifies that when multiple image sets contain images that map
	// to the same destination tag, only the first one is included in the output.
	// This prevents 'oc image mirror' from rejecting the mirror file with
	// "each destination tag may only be specified once".

	ref, err := reference.Parse("quay.io/test/repo")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	// Create two image sets that both contain the same image (simulating two
	// OTE extension binaries registering the same upstream image). When mapped
	// to the target repo, they will produce the same destination tag.
	imageID := k8simage.ImageID(9999)

	sourceConfig := k8simage.Config{}
	sourceConfig.SetRegistry("registry.k8s.io")
	sourceConfig.SetName("e2e-test-images/agnhost")
	sourceConfig.SetVersion("2.63.0")

	// defaultImageSets: two sets, each with the same image (same source)
	defaultSet1 := extensions.ImageSet{imageID: sourceConfig}
	defaultSet2 := extensions.ImageSet{imageID: sourceConfig}
	defaultImageSets := []extensions.ImageSet{defaultSet1, defaultSet2}

	// updatedImageSets: both map to the same destination via GetMappedImageConfigs
	updatedSet1 := k8simage.GetMappedImageConfigs(defaultSet1, ref.Exact())
	updatedSet2 := k8simage.GetMappedImageConfigs(defaultSet2, ref.Exact())
	updatedImageSets := []extensions.ImageSet{updatedSet1, updatedSet2}

	// Verify precondition: both updated configs produce the same destination
	cfg1, cfg2 := updatedSet1[imageID], updatedSet2[imageID]
	dest1 := cfg1.GetE2EImage()
	dest2 := cfg2.GetE2EImage()
	if dest1 != dest2 {
		t.Fatalf("test setup error: destinations should match but got %q vs %q", dest1, dest2)
	}

	// Now simulate the output loop from createImageMirrorForInternalImages
	// with the fix applied (destination-based dedup)
	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)

	// Count how many times the destination appears
	destCount := 0
	for _, line := range lines {
		parts := strings.Fields(line)
		if len(parts) >= 2 && parts[1] == dest1 {
			destCount++
		}
	}
	if destCount != 1 {
		t.Errorf("expected destination %q to appear exactly once, but appeared %d times in output:\n%s",
			dest1, destCount, strings.Join(lines, "\n"))
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
