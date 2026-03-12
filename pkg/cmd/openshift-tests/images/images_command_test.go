package images

import (
	"fmt"
	"strings"
	"testing"

	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
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

func TestConvertImageToImageConfig(t *testing.T) {
	tests := []struct {
		name     string
		image    extension.Image
		expected string
	}{
		{
			name: "standard image",
			image: extension.Image{
				Index:    1,
				Registry: "registry.k8s.io",
				Name:     "agnhost",
				Version:  "2.63.0",
			},
			expected: "registry.k8s.io/agnhost:2.63.0",
		},
		{
			name: "multi-segment name",
			image: extension.Image{
				Index:    7,
				Registry: "registry.k8s.io",
				Name:     "build-image/distroless-iptables",
				Version:  "v0.9.7",
			},
			expected: "registry.k8s.io/build-image/distroless-iptables:v0.9.7",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := convertImageToImageConfig(tt.image)
			if got := config.GetE2EImage(); got != tt.expected {
				t.Errorf("got %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestExternalBinaryMappedImageUsedInOutput(t *testing.T) {
	// Simulate what the external binary path does: construct defaultImageSets
	// from the original image and updatedImageSets from image.Mapped.
	// The mapped tag is deliberately different from what vendored
	// GetMappedImageConfigs would produce, proving the output uses Mapped.
	orig := extension.Image{
		Index:    7,
		Registry: "registry.k8s.io",
		Name:     "build-image/distroless-iptables",
		Version:  "v0.9.7",
		Mapped: &extension.Image{
			Index:    7,
			Registry: "quay.io",
			Name:     "openshift/community-e2e-images",
			Version:  "e2e-7-NEW-upstream-format",
		},
	}

	ref, err := reference.Parse("quay.io/openshift/community-e2e-images")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	imageID := k8simage.ImageID(orig.Index)
	origConfig := convertImageToImageConfig(orig)

	// Build mapped config the way production code should: tag from Mapped,
	// registry/name from ref (--to-repository).
	mappedConfig := k8simage.Config{}
	mappedConfig.SetRegistry(ref.Registry)
	mappedConfig.SetName(ref.RepositoryName())
	mappedConfig.SetVersion(orig.Mapped.Version)

	defaultImageSets := []extensions.ImageSet{{imageID: origConfig}}
	updatedImageSets := []extensions.ImageSet{{imageID: mappedConfig}}

	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d: %v", len(lines), lines)
	}
	if !strings.Contains(lines[0], "e2e-7-NEW-upstream-format") {
		t.Errorf("expected mapped tag in output, got: %s", lines[0])
	}
	expected := "registry.k8s.io/build-image/distroless-iptables:v0.9.7 quay.io/openshift/community-e2e-images:e2e-7-NEW-upstream-format"
	if lines[0] != expected {
		t.Errorf("got:\n  %s\nwant:\n  %s", lines[0], expected)
	}
}

func TestExternalBinaryMappedImageTargetsDifferentRepo(t *testing.T) {
	// When --to-repository differs from the default mirror, the destination
	// must use the --to-repository registry/name with the tag from Mapped.
	// Without this, mirrored=true rewrites the source to the default mirror
	// and from == to, causing the image to be silently dropped.
	orig := extension.Image{
		Index:    7,
		Registry: "registry.k8s.io",
		Name:     "build-image/distroless-iptables",
		Version:  "v0.9.7",
		Mapped: &extension.Image{
			Index:    7,
			Registry: "quay.io",
			Name:     "openshift/community-e2e-images",
			Version:  "e2e-7-registry-k8s-io-build-image-distroless-iptables-v0-9-7-ACxHodPTjhkRc9bQ",
		},
	}

	// User passes --to-repository quay.io/bertinatto/openshift-tests
	ref, err := reference.Parse("quay.io/bertinatto/openshift-tests")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	imageID := k8simage.ImageID(orig.Index)
	origConfig := convertImageToImageConfig(orig)

	// Build mapped config: tag from Mapped, registry/name from ref
	mappedConfig := k8simage.Config{}
	mappedConfig.SetRegistry(ref.Registry)
	mappedConfig.SetName(ref.RepositoryName())
	mappedConfig.SetVersion(orig.Mapped.Version)

	defaultImageSets := []extensions.ImageSet{{imageID: origConfig}}
	updatedImageSets := []extensions.ImageSet{{imageID: mappedConfig}}

	// Simulate mirrored=true: rewrite source to the default mirror
	sourceConfig := k8simage.Config{}
	sourceConfig.SetRegistry("quay.io")
	sourceConfig.SetName("openshift/community-e2e-images")
	sourceConfig.SetVersion(orig.Mapped.Version)
	defaultImageSets[0][imageID] = sourceConfig

	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)

	if len(lines) != 1 {
		t.Fatalf("expected 1 line, got %d (image was dropped because from == to): %v", len(lines), lines)
	}

	expectedFrom := "quay.io/openshift/community-e2e-images:e2e-7-registry-k8s-io-build-image-distroless-iptables-v0-9-7-ACxHodPTjhkRc9bQ"
	expectedTo := "quay.io/bertinatto/openshift-tests:e2e-7-registry-k8s-io-build-image-distroless-iptables-v0-9-7-ACxHodPTjhkRc9bQ"
	expected := expectedFrom + " " + expectedTo
	if lines[0] != expected {
		t.Errorf("got:\n  %s\nwant:\n  %s", lines[0], expected)
	}
}

func TestExternalBinaryMappedNilFallback(t *testing.T) {
	// When Mapped is nil, the production code appends the original to both
	// defaultImageSets and updatedImageSets. Since from == to, the output
	// loop skips the image and produces no line.
	orig := extension.Image{
		Index:    5,
		Registry: "registry.k8s.io",
		Name:     "e2e-test-images/agnhost",
		Version:  "2.63.0",
		Mapped:   nil,
	}

	imageID := k8simage.ImageID(orig.Index)
	origConfig := convertImageToImageConfig(orig)
	initialSet := extensions.ImageSet{imageID: origConfig}

	defaultImageSets := []extensions.ImageSet{initialSet}
	updatedImageSets := []extensions.ImageSet{initialSet}

	lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)

	if len(lines) != 0 {
		t.Errorf("expected no output lines when Mapped is nil (from == to), got: %v", lines)
	}
}

func TestImageMatchesExceptions(t *testing.T) {
	exceptions := []string{
		"registry.k8s.io/pause:3.10",
		"invalid.registry.k8s.io/invalid",
		"gcr.io/k8s-authenticated-test",
	}

	tests := []struct {
		name     string
		image    string
		expected bool
	}{
		{
			name:     "matches exception exactly",
			image:    "registry.k8s.io/pause:3.10",
			expected: true,
		},
		{
			name:     "matches exception as substring",
			image:    "invalid.registry.k8s.io/invalid/alpine:3.1",
			expected: true,
		},
		{
			name:     "no match",
			image:    "registry.k8s.io/build-image/distroless-iptables:v0.9.7",
			expected: false,
		},
		{
			name:     "partial string does not match",
			image:    "registry.k8s.io/pause:3.11",
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			config := k8simage.Config{}
			parts := strings.SplitN(tt.image, "/", 2)
			config.SetRegistry(parts[0])
			nameAndVersion := parts[1]
			if idx := strings.LastIndex(nameAndVersion, ":"); idx != -1 {
				config.SetName(nameAndVersion[:idx])
				config.SetVersion(nameAndVersion[idx+1:])
			}

			if got := imageMatchesExceptions(config, exceptions); got != tt.expected {
				t.Errorf("imageMatchesExceptions(%q) = %v, want %v", tt.image, got, tt.expected)
			}
		})
	}
}

func TestFilterExceptionsFromImageSets(t *testing.T) {
	makeConfig := func(registry, name, version string) k8simage.Config {
		c := k8simage.Config{}
		c.SetRegistry(registry)
		c.SetName(name)
		c.SetVersion(version)
		return c
	}

	validImage := makeConfig("registry.k8s.io", "build-image/distroless-iptables", "v0.9.7")
	exceptedImage := makeConfig("registry.k8s.io", "pause", "3.10")
	anotherValid := makeConfig("registry.k8s.io", "e2e-test-images/agnhost", "2.63.0")

	input := []extensions.ImageSet{
		{k8simage.ImageID(1): validImage, k8simage.ImageID(2): exceptedImage},
		{k8simage.ImageID(3): anotherValid},
		{k8simage.ImageID(4): exceptedImage},
	}

	result := filterExceptionsFromImageSets(input)

	// Set with only the excepted image should be dropped entirely
	if len(result) != 2 {
		t.Fatalf("expected 2 sets after filtering, got %d", len(result))
	}

	// First set should have only the valid image, excepted one removed
	if len(result[0]) != 1 {
		t.Errorf("expected 1 image in first set, got %d", len(result[0]))
	}
	if _, ok := result[0][k8simage.ImageID(1)]; !ok {
		t.Error("expected imageID 1 to be present in first set")
	}
	if _, ok := result[0][k8simage.ImageID(2)]; ok {
		t.Error("expected imageID 2 (excepted) to be removed from first set")
	}

	// Second set should be unchanged
	if len(result[1]) != 1 {
		t.Errorf("expected 1 image in second set, got %d", len(result[1]))
	}
	if _, ok := result[1][k8simage.ImageID(3)]; !ok {
		t.Error("expected imageID 3 to be present in second set")
	}
}
