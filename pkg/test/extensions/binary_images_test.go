package extensions

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	k8simage "k8s.io/kubernetes/test/utils/image"
)

func TestListImagesPreservesImages(t *testing.T) {
	tests := []struct {
		name   string
		images []Image
	}{
		{name: "null images"},
		{name: "empty images", images: []Image{}},
		{
			name: "custom images sharing index zero",
			images: []Image{
				{Index: 0, Registry: "example.invalid", Name: "repro/first", Version: "v1"},
				{Index: 0, Registry: "example.invalid", Name: "repro/second", Version: "v1"},
			},
		},
		{
			name: "images sharing a nonzero index",
			images: []Image{
				{Index: 9999, Registry: "example.invalid", Name: "repro/first", Version: "v1"},
				{Index: 9999, Registry: "example.invalid", Name: "repro/second", Version: "v1"},
			},
		},
		{
			name: "identical images remain in advertised order",
			images: []Image{
				{Index: 0, Registry: "example.invalid", Name: "repro/first", Version: "v1"},
				{Index: 0, Registry: "example.invalid", Name: "repro/first", Version: "v1"},
			},
		},
		{
			name: "mixed custom and indexed images",
			images: []Image{
				{Index: 0, Registry: "example.invalid", Name: "repro/first", Version: "v1"},
				{Index: int(k8simage.Agnhost), Registry: "registry.k8s.io/e2e-test-images", Name: "agnhost", Version: "2.53"},
				{Index: 0, Registry: "example.invalid", Name: "repro/second", Version: "v1"},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			data, err := json.Marshal(tt.images)
			if err != nil {
				t.Fatal(err)
			}
			binaryPath := filepath.Join(t.TempDir(), "extension")
			script := "#!/bin/sh\n[ \"$1\" = images ] || exit 1\ncat <<'JSON'\n" + string(data) + "\nJSON\n"
			if err := os.WriteFile(binaryPath, []byte(script), 0755); err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			// Concurrent collection must retain one ordered set per extension.
			binaries := TestBinaries{&TestBinary{binaryPath: binaryPath}, &TestBinary{binaryPath: binaryPath}}
			imageSets, err := binaries.ListImages(ctx, 2)
			if err != nil {
				t.Fatal(err)
			}

			if len(imageSets) != len(binaries) {
				t.Fatalf("collected %d image sets, want %d", len(imageSets), len(binaries))
			}
			for _, imageSet := range imageSets {
				if len(imageSet) != len(tt.images) {
					t.Fatalf("collected %d images, want %d", len(imageSet), len(tt.images))
				}
				for i, image := range tt.images {
					config := imageSet[i]
					pullSpec := image.Registry + "/" + image.Name + ":" + image.Version
					if got := config.GetE2EImage(); got != pullSpec {
						t.Errorf("image at position %d = %s, want %s", i, got, pullSpec)
					}
					if config.K8sIndex != k8simage.ImageID(image.Index) {
						t.Errorf("image %s: Kubernetes index = %d, want %d", pullSpec, config.K8sIndex, image.Index)
					}
				}
			}
		})
	}
}
