package images

import (
	"strings"
	"testing"

	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
	"github.com/openshift/library-go/pkg/image/reference"
)

func TestCreateImageMirrorDeduplicatesByDestination(t *testing.T) {
	// This test verifies that when multiple mappings contain images with
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

	// Source for mapping 2: the upstream image.
	upstreamConfig := k8simage.Config{}
	upstreamConfig.SetRegistry("registry.k8s.io")
	upstreamConfig.SetName("e2e-test-images/agnhost")
	upstreamConfig.SetVersion("2.63.0")

	// Map the upstream config to derive the canonical destination.
	mappedSet := k8simage.GetMappedImageConfigs(
		map[k8simage.ImageID]k8simage.Config{imageID: upstreamConfig}, ref.Exact(),
	)
	destConfig := mappedSet[imageID]

	// Source for mapping 1: a DIFFERENT source image (simulating a pre-mapped
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

	dest := destConfig.GetE2EImage()
	mappings := []imageMapping{
		{source: from1, destination: dest},
		{source: from2, destination: dest},
	}
	lines := deduplicatedMirrorLines("", mappings)

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

func TestCreateImageMirrorDeduplicatesAcrossSources(t *testing.T) {
	// A mapping from an Origin image must not duplicate an extension mapping's
	// destination.

	ref, err := reference.Parse("quay.io/test/repo")
	if err != nil {
		t.Fatalf("failed to parse reference: %v", err)
	}

	imageID := k8simage.ImageID(9999)

	sourceConfig := k8simage.Config{}
	sourceConfig.SetRegistry("registry.k8s.io")
	sourceConfig.SetName("e2e-test-images/agnhost")
	sourceConfig.SetVersion("2.63.0")

	updatedSet := k8simage.GetMappedImageConfigs(
		map[k8simage.ImageID]k8simage.Config{imageID: sourceConfig}, ref.Exact(),
	)

	cfg := updatedSet[imageID]
	destTag := cfg.GetE2EImage()

	mappings := []imageMapping{
		{source: sourceConfig.GetE2EImage(), destination: destTag},
		{source: "quay.io/openshift/community-e2e-images:different-source", destination: destTag},
	}

	lines := deduplicatedMirrorLines("", mappings)

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

func TestMappingForExtensionImageOverlappingIndexes(t *testing.T) {
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
				var mappings []imageMapping
				wantLines := map[string]bool{}
				for _, i := range order {
					m := mappingForExtensionImage(images[i], ref)
					mappings = append(mappings, m)

					id := k8simage.ImageID(images[i].Index)
					source := images[i].Registry + "/" + images[i].Name + ":" + images[i].Version
					if m.source != source {
						t.Fatalf("unexpected source: %s, want %s", m.source, source)
					}
					config := k8simage.Config{}
					config.SetRegistry(images[i].Registry)
					config.SetName(images[i].Name)
					config.SetVersion(images[i].Version)
					fallback := k8simage.GetMappedImageConfigs(map[k8simage.ImageID]k8simage.Config{id: config}, ref.Exact())[id]
					dest := fallback.GetE2EImage()
					if tc.tags[i] != "" {
						dest = ref.Exact() + ":" + tc.tags[i]
					}
					wantLines[source+" "+dest] = true
				}
				lines := deduplicatedMirrorLines("", mappings)
				if len(lines) != len(images) {
					t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(images), strings.Join(lines, "\n"))
				}
				for _, line := range lines {
					if !wantLines[line] {
						t.Errorf("unexpected or duplicate mirror line: %s", line)
						continue
					}
					delete(wantLines, line)
				}
				for line := range wantLines {
					t.Errorf("missing mirror line: %s", line)
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

	for _, tc := range []struct {
		name string
		dest []k8simage.Config
	}{
		{"old first", []k8simage.Config{oldTag, newTag}},
		{"new first", []k8simage.Config{newTag, oldTag}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			mappings := []imageMapping{
				{source: source, destination: tc.dest[0].GetE2EImage()},
				{source: source, destination: tc.dest[1].GetE2EImage()},
			}
			lines := deduplicatedMirrorLines("", mappings)
			want := map[string]bool{
				source + " " + oldTag.GetE2EImage(): true,
				source + " " + newTag.GetE2EImage(): true,
			}
			if len(lines) != len(want) {
				t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
			}
			for _, line := range lines {
				if !want[line] {
					t.Errorf("unexpected or duplicate mirror line: %s", line)
					continue
				}
				delete(want, line)
			}
			for line := range want {
				t.Errorf("missing mirror line: %s", line)
			}
		})
	}

	// The same rule holds when one of the mappings comes from the static
	// openshift image path.
	t.Run("static image path", func(t *testing.T) {
		mappings := []imageMapping{
			{source: source, destination: newTag.GetE2EImage()},
			{source: source, destination: oldTag.GetE2EImage()},
		}
		lines := deduplicatedMirrorLines("", mappings)
		want := map[string]bool{
			source + " " + oldTag.GetE2EImage(): true,
			source + " " + newTag.GetE2EImage(): true,
		}
		if len(lines) != len(want) {
			t.Fatalf("got %d mirror lines, want %d:\n%s", len(lines), len(want), strings.Join(lines, "\n"))
		}
		for _, line := range lines {
			if !want[line] {
				t.Errorf("unexpected or duplicate mirror line: %s", line)
				continue
			}
			delete(want, line)
		}
		for line := range want {
			t.Errorf("missing mirror line: %s", line)
		}
	})
}
