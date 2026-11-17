package images

import (
	"fmt"
	"reflect"
	"slices"
	"sort"
	"strings"
	"testing"

	k8simage "k8s.io/kubernetes/test/utils/image"

	"github.com/openshift/library-go/pkg/image/reference"
	"github.com/openshift/origin/pkg/clioptions/imagesetup"
	"github.com/openshift/origin/pkg/test/extensions"
	"github.com/openshift/origin/test/extended/util/image"
)

func TestCreateImageMirrorForImageSets(t *testing.T) {
	first := testImageConfig(k8simage.None, "example.invalid", "repro/first", "v1")
	second := testImageConfig(k8simage.None, "example.invalid", "repro/second", "v1")
	indexedFirst, indexedSecond := first, second
	indexedFirst.K8sIndex = 9999
	indexedSecond.K8sIndex = 9999
	indexed := testImageConfig(k8simage.Agnhost, "example.invalid", "repro/kubernetes", "v1")
	unmapped := testImageConfig(k8simage.InvalidRegistryImage, "example.invalid", "repro/unmapped", "v1")
	invalid := testImageConfig(k8simage.InvalidRegistryImage, "invalid.registry.k8s.io", "invalid", "latest")
	private := testImageConfig(k8simage.AgnhostPrivate, "gcr.io", "k8s-authenticated-test/agnhost", "2.53")
	pause := testImageConfig(k8simage.Pause, "registry.k8s.io", "pause", "3.10")
	tests := []struct {
		name   string
		sets   []extensions.ImageSet
		images extensions.ImageSet
	}{
		{name: "no image sets"},
		{name: "empty image set", sets: []extensions.ImageSet{{}}},
		{
			name:   "custom images sharing index zero",
			sets:   []extensions.ImageSet{{first, second}},
			images: extensions.ImageSet{first, second},
		},
		{
			name:   "images sharing a nonzero index",
			sets:   []extensions.ImageSet{{indexedFirst, indexedSecond}},
			images: extensions.ImageSet{indexedFirst, indexedSecond},
		},
		{
			name:   "mixed images with exceptions between them",
			sets:   []extensions.ImageSet{{invalid, first, pause, indexed, private, second}},
			images: extensions.ImageSet{first, indexed, second},
		},
		{
			name: "only exceptions",
			sets: []extensions.ImageSet{{invalid, private, pause}},
		},
		{
			name:   "duplicate images within and across extensions",
			sets:   []extensions.ImageSet{{first, first, second}, {second, first}},
			images: extensions.ImageSet{first, second},
		},
		{name: "special Kubernetes index remains unmapped", sets: []extensions.ImageSet{{unmapped}}},
	}
	for _, tt := range tests {
		for _, mirrored := range []bool{false, true} {
			for _, prefix := range []string{"", "file://"} {
				t.Run(fmt.Sprintf("%s/mirrored=%t/prefix=%s", tt.name, mirrored, prefix), func(t *testing.T) {
					const repo = "mirror.example.invalid/e2e"
					ref, err := reference.Parse(repo)
					if err != nil {
						t.Fatal(err)
					}
					setsBefore := make([]extensions.ImageSet, len(tt.sets))
					for i, set := range tt.sets {
						setsBefore[i] = slices.Clone(set)
					}
					lines := createImageMirrorForImageSets(prefix, ref, mirrored, tt.sets)
					if !sort.StringsAreSorted(lines) {
						t.Errorf("mirror lines are not sorted: %v", lines)
					}
					for i := range tt.sets {
						if !reflect.DeepEqual(tt.sets[i], setsBefore[i]) {
							t.Errorf("image set %d was modified", i)
						}
					}

					// Static Origin images are also present; compare the extension-only additions.
					baseline := createImageMirrorForImageSets(prefix, ref, mirrored, nil)
					var got []string
					for _, line := range lines {
						if !slices.Contains(baseline, line) {
							got = append(got, line)
						}
					}
					var want []string
					for _, config := range tt.images {
						source := config.GetE2EImage()
						t.Setenv("KUBE_TEST_REPO", repo)
						var target string
						if config.K8sIndex == k8simage.None {
							// Custom images must match the runtime registry replacement helper.
							target, err = k8simage.ReplaceRegistryInImageURL(source)
							if err != nil {
								t.Fatal(err)
							}
						} else {
							original := map[k8simage.ImageID]k8simage.Config{config.K8sIndex: config.Config}
							mapped := k8simage.GetMappedImageConfigs(original, repo)
							mappedConfig := mapped[config.K8sIndex]
							target = mappedConfig.GetE2EImage()
							if !strings.Contains(target, fmt.Sprintf(":e2e-%d-", config.K8sIndex)) {
								t.Fatalf("target %s does not preserve Kubernetes index %d", target, config.K8sIndex)
							}
						}
						if mirrored {
							_, tag, ok := strings.Cut(target, ":")
							if !ok {
								t.Fatalf("target has no tag: %s", target)
							}
							source = imagesetup.DefaultTestImageMirrorLocation + ":" + tag
						}
						want = append(want, source+" "+prefix+target)
					}
					sort.Strings(want)
					if !reflect.DeepEqual(got, want) {
						t.Errorf("extension mirror lines = %v, want %v", got, want)
					}
				})
			}
		}
	}
}

func TestCreateImageMirrorDeduplicatesStaticImages(t *testing.T) {
	config := testImageConfig(k8simage.None, "quay.io", "kubevirt/fedora-with-test-tooling-container-disk", "v1.8.2")
	if _, exists := image.OriginalImages()[config.GetE2EImage()]; !exists {
		t.Fatal("test image must also be in Origin's static image list")
	}
	const repo = "mirror.example.invalid/e2e"
	ref, err := reference.Parse(repo)
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("KUBE_TEST_REPO", repo)
	target, err := k8simage.ReplaceRegistryInImageURL(config.GetE2EImage())
	if err != nil {
		t.Fatal(err)
	}
	for _, mirrored := range []bool{false, true} {
		t.Run(fmt.Sprintf("mirrored=%t", mirrored), func(t *testing.T) {
			from := config.GetE2EImage()
			if mirrored {
				_, tag, _ := strings.Cut(target, ":")
				from = imagesetup.DefaultTestImageMirrorLocation + ":" + tag
			}
			lines := createImageMirrorForImageSets("file://", ref, mirrored, []extensions.ImageSet{{config}})
			want := from + " file://" + target
			if !slices.Contains(lines, want) {
				t.Errorf("missing extension mapping %q", want)
			}
			var sources, destinations int
			for _, line := range lines {
				fields := strings.Fields(line)
				if fields[0] == from {
					sources++
				}
				if fields[1] == "file://"+target {
					destinations++
				}
			}
			if sources != 1 || destinations != 1 {
				t.Errorf("extension/static overlap: source appears %d times, destination %d times; want once each", sources, destinations)
			}
		})
	}
}

func TestMirrorMappings(t *testing.T) {
	tests := []struct {
		name     string
		mappings [][2]string
		want     []string
	}{
		{name: "empty mappings"},
		{
			name:     "identical mappings",
			mappings: [][2]string{{"upstream:first", "mirror:target"}, {"upstream:first", "mirror:target"}},
			want:     []string{"upstream:first mirror:target"},
		},
		{
			name:     "different sources with the same destination",
			mappings: [][2]string{{"community:first", "mirror:target"}, {"upstream:second", "mirror:target"}},
			want:     []string{"community:first mirror:target"},
		},
		{
			name:     "same source with different destinations",
			mappings: [][2]string{{"upstream:first", "mirror:first"}, {"upstream:first", "mirror:second"}},
			want:     []string{"upstream:first mirror:first"},
		},
		{
			name:     "rejected destination does not reserve its source",
			mappings: [][2]string{{"upstream:first", "mirror:first"}, {"upstream:second", "mirror:first"}, {"upstream:second", "mirror:second"}},
			want:     []string{"upstream:first mirror:first", "upstream:second mirror:second"},
		},
		{
			name:     "rejected source does not reserve its destination",
			mappings: [][2]string{{"upstream:first", "mirror:first"}, {"upstream:first", "mirror:second"}, {"upstream:second", "mirror:second"}},
			want:     []string{"upstream:first mirror:first", "upstream:second mirror:second"},
		},
		{
			name:     "unchanged images are skipped without reserving entries",
			mappings: [][2]string{{"upstream:first", "upstream:first"}, {"upstream:first", "mirror:target"}, {"upstream:second", "upstream:second"}},
			want:     []string{"upstream:first mirror:target"},
		},
		{
			name:     "output is sorted by source rather than destination",
			mappings: [][2]string{{"upstream:z", "mirror:a"}, {"upstream:a", "mirror:z"}},
			want:     []string{"upstream:a mirror:z", "upstream:z mirror:a"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var mappings mirrorMappings
			for _, mapping := range tt.mappings {
				mappings.Add(mapping[0], mapping[1])
			}
			for _, prefix := range []string{"", "file://", "s3://"} {
				var want []string
				for _, line := range tt.want {
					want = append(want, strings.Replace(line, " ", " "+prefix, 1))
				}
				got := mappings.Lines(prefix)
				if !reflect.DeepEqual(got, want) {
					t.Errorf("mirror lines with prefix %q = %v, want %v", prefix, got, want)
				}
			}
			if got := mappings.Lines(""); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("rendering lines modified mappings: got %v, want %v", got, tt.want)
			}
		})
	}
}

func testImageConfig(index k8simage.ImageID, registry, name, version string) extensions.ImageConfig {
	config := extensions.ImageConfig{K8sIndex: index}
	config.SetRegistry(registry)
	config.SetName(name)
	config.SetVersion(version)
	return config
}
