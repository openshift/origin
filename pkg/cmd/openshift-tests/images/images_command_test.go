package images

import (
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
	// Using distinct sources is critical: identical sources may now map to
	// different destinations, so only the destination dedup can suppress
	// the duplicate.

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

func TestImageSetsForImageOverlappingIndexes(t *testing.T) {
	ref, err := reference.Parse("private.example.com/tests/images")
	if err != nil {
		t.Fatal(err)
	}

	for _, tc := range []struct {
		name string
		tags []string
	}{
		{name: "fallback tags", tags: []string{"", "", "", ""}},
		{name: "mixed reported and fallback tags", tags: []string{"agnhost-tag", "fedora-tag", "", ""}},
		{name: "reported tags", tags: []string{"agnhost-tag", "fedora-tag", "netshoot-tag", "pause-tag"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			// The first three records are from the OVN binary; pause is from CSO.
			images := []extension.Image{
				{Index: 2, Registry: "registry.k8s.io", Name: "e2e-test-images/agnhost", Version: "2.63.0"},
				{Index: 0, Registry: "quay.io", Name: "kubevirt/fedora-with-test-tooling-container-disk", Version: "v1.8.2"},
				{Index: 0, Registry: "ghcr.io", Name: "nicolaka/netshoot", Version: "v0.16"},
				{Index: 0, Registry: "k8s.gcr.io", Name: "pause", Version: "3.2"},
			}
			for i, tag := range tc.tags {
				if tag != "" {
					images[i].Mapped = &extension.Image{Version: tag}
				}
			}
			for _, order := range [][]int{{0, 1, 2, 3}, {3, 2, 1, 0}} {
				var originals, mapped []extensions.ImageSet
				wantLines := map[string]bool{}
				for _, i := range order {
					original, updated := imageSetsForImage(images[i], ref)
					originals = append(originals, original)
					mapped = append(mapped, updated)

					id := k8simage.ImageID(images[i].Index)
					source := images[i].Registry + "/" + images[i].Name + ":" + images[i].Version
					config := original[id]
					if config.GetE2EImage() != source {
						t.Fatalf("unexpected original: %s, want %s", config.GetE2EImage(), source)
					}
					fallback := k8simage.GetMappedImageConfigs(original, ref.Exact())[id]
					dest := fallback.GetE2EImage()
					if tc.tags[i] != "" {
						dest = ref.Exact() + ":" + tc.tags[i]
					}
					wantLines[source+" "+dest] = true
				}
				lines := deduplicatedMirrorLines("", originals, mapped, nil)
				if len(lines) != len(images) {
					t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(images), strings.Join(lines, "\n"))
				}
				for _, line := range lines {
					if !wantLines[line] {
						t.Errorf("unexpected mirror line: %s", line)
					}
				}
			}
		})
	}
}

func TestCreateImageMirrorKeepsDistinctDestinationsForSameSource(t *testing.T) {
	// Multiple extensions can report the same upstream image but require
	// different mirror tags (e.g. their mapping tools generated different
	// tags). Every required destination must be emitted; deduplication is by
	// destination only.
	ref, err := reference.Parse("quay.io/test/repo")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	sourceConfig := k8simage.Config{}
	sourceConfig.SetRegistry("registry.k8s.io")
	sourceConfig.SetName("e2e-test-images/agnhost")
	sourceConfig.SetVersion("2.63.0")
	source := sourceConfig.GetE2EImage()

	destFor := func(tag string) k8simage.Config {
		config := k8simage.Config{}
		config.SetRegistry(ref.Registry)
		config.SetName(ref.RepositoryName())
		config.SetVersion(tag)
		return config
	}
	oldTag, newTag := destFor("e2e-2-old-tag"), destFor("e2e-2-new-tag")

	imageID := k8simage.ImageID(2)
	for _, tc := range []struct {
		name string
		dest []k8simage.Config
	}{
		{"old first", []k8simage.Config{oldTag, newTag}},
		{"new first", []k8simage.Config{newTag, oldTag}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			defaultImageSets := []extensions.ImageSet{
				{imageID: sourceConfig},
				{imageID: sourceConfig},
			}
			updatedImageSets := []extensions.ImageSet{
				{imageID: tc.dest[0]},
				{imageID: tc.dest[1]},
			}
			lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, nil)
			want := map[string]bool{
				source + " " + oldTag.GetE2EImage(): true,
				source + " " + newTag.GetE2EImage(): true,
			}
			if len(lines) != len(want) {
				t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
			}
			for _, line := range lines {
				if !want[line] {
					t.Errorf("unexpected mirror line: %s", line)
				}
			}
		})
	}

	// The same rule holds when one of the mappings comes from the static
	// openshift image path.
	t.Run("static image path", func(t *testing.T) {
		defaultImageSets := []extensions.ImageSet{{imageID: sourceConfig}}
		updatedImageSets := []extensions.ImageSet{{imageID: newTag}}
		openshiftUpdated := map[string]string{source: oldTag.GetE2EImage()}
		lines := deduplicatedMirrorLines("", defaultImageSets, updatedImageSets, openshiftUpdated)
		want := map[string]bool{
			source + " " + oldTag.GetE2EImage(): true,
			source + " " + newTag.GetE2EImage(): true,
		}
		if len(lines) != len(want) {
			t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
		}
		for _, line := range lines {
			if !want[line] {
				t.Errorf("unexpected mirror line: %s", line)
			}
		}
	})
}
