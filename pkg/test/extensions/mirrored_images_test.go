package extensions

import (
	"os"
	"testing"

	"github.com/openshift-eng/openshift-tests-extension/pkg/extension"
)

func TestComputeMirroredImages(t *testing.T) {
	tests := []struct {
		name           string
		extensions     []*Extension
		fromRepository string
		wantCount      int
		wantMapping    map[string]string
		wantErr        bool
	}{
		{
			name:           "empty from-repository returns nil",
			extensions:     []*Extension{},
			fromRepository: "",
			wantCount:      0,
		},
		{
			name:           "no extensions returns nil",
			extensions:     []*Extension{},
			fromRepository: "quay.io/openshift/community-e2e-images",
			wantCount:      0,
		},
		{
			name: "nil extension is skipped",
			extensions: []*Extension{
				nil,
			},
			fromRepository: "quay.io/openshift/community-e2e-images",
			wantCount:      0,
		},
		{
			name: "image without Mapped uses computed tag",
			extensions: []*Extension{
				{
					Extension: &extension.Extension{
						Images: []extension.Image{
							{
								Index:    100,
								Registry: "registry.k8s.io",
								Name:     "e2e-test-images/agnhost",
								Version:  "2.55",
							},
						},
					},
				},
			},
			fromRepository: "private.com/test/repo",
			wantCount:      1,
		},
		{
			name: "image with Mapped uses mapped version",
			extensions: []*Extension{
				{
					Extension: &extension.Extension{
						Images: []extension.Image{
							{
								Index:    200,
								Registry: "registry.k8s.io",
								Name:     "e2e-test-images/busybox",
								Version:  "1.37.0-2",
								Mapped: &extension.Image{
									Version: "custom-tag-v1",
								},
							},
						},
					},
				},
			},
			fromRepository: "private.com/test/repo",
			wantCount:      1,
			wantMapping: map[string]string{
				"registry.k8s.io/e2e-test-images/busybox:1.37.0-2": "private.com/test/repo:custom-tag-v1",
			},
		},
		{
			name: "multiple extensions aggregate images",
			extensions: []*Extension{
				{
					Extension: &extension.Extension{
						Images: []extension.Image{
							{
								Index:    1,
								Registry: "quay.io",
								Name:     "test/image-a",
								Version:  "v1",
							},
						},
					},
				},
				{
					Extension: &extension.Extension{
						Images: []extension.Image{
							{
								Index:    2,
								Registry: "quay.io",
								Name:     "test/image-b",
								Version:  "v2",
							},
						},
					},
				},
			},
			fromRepository: "private.com/mirror",
			wantCount:      2,
		},
		{
			name:           "invalid from-repository returns error",
			extensions:     []*Extension{},
			fromRepository: "://invalid",
			wantErr:        true,
		},
		{
			name:           "from-repository without registry/namespace returns error",
			extensions:     []*Extension{},
			fromRepository: "mirror",
			wantErr:        true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mapping, err := ComputeMirroredImages(tt.extensions, tt.fromRepository)
			if (err != nil) != tt.wantErr {
				t.Errorf("ComputeMirroredImages() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}
			if len(mapping) != tt.wantCount {
				t.Errorf("ComputeMirroredImages() returned %d mappings, want %d", len(mapping), tt.wantCount)
			}
			for original, expected := range tt.wantMapping {
				if actual, ok := mapping[original]; !ok {
					t.Errorf("missing mapping for %q", original)
				} else if actual != expected {
					t.Errorf("mapping[%q] = %q, want %q", original, actual, expected)
				}
			}
		})
	}
}

func TestWriteAndReadMirroredImagesFile(t *testing.T) {
	mapping := map[string]string{
		"registry.k8s.io/e2e-test-images/agnhost:2.55": "private.com/repo:e2e-1-abc",
		"quay.io/test/image:v1":                        "private.com/repo:e2e-custom",
	}

	path, err := WriteMirroredImagesFile(mapping)
	if err != nil {
		t.Fatalf("WriteMirroredImagesFile() error = %v", err)
	}
	defer os.Remove(path)

	if path == "" {
		t.Fatal("WriteMirroredImagesFile() returned empty path")
	}

	read, err := ReadMirroredImagesFile(path)
	if err != nil {
		t.Fatalf("ReadMirroredImagesFile() error = %v", err)
	}

	if len(read) != len(mapping) {
		t.Errorf("read %d mappings, want %d", len(read), len(mapping))
	}

	for k, v := range mapping {
		if read[k] != v {
			t.Errorf("read[%q] = %q, want %q", k, read[k], v)
		}
	}
}

func TestWriteMirroredImagesFileEmpty(t *testing.T) {
	path, err := WriteMirroredImagesFile(nil)
	if err != nil {
		t.Fatalf("WriteMirroredImagesFile(nil) error = %v", err)
	}
	if path != "" {
		t.Errorf("WriteMirroredImagesFile(nil) = %q, want empty", path)
	}
}

func TestMirroredLocationFor(t *testing.T) {
	mapping := map[string]string{
		"registry.k8s.io/e2e-test-images/agnhost:2.55": "private.com/repo:e2e-1-abc",
	}

	t.Run("mapped image returns mirrored pullspec", func(t *testing.T) {
		got := MirroredLocationFor(mapping, "registry.k8s.io/e2e-test-images/agnhost:2.55")
		want := "private.com/repo:e2e-1-abc"
		if got != want {
			t.Errorf("MirroredLocationFor() = %q, want %q", got, want)
		}
	})

	t.Run("unmapped image returns original", func(t *testing.T) {
		got := MirroredLocationFor(mapping, "quay.io/unknown/image:v1")
		want := "quay.io/unknown/image:v1"
		if got != want {
			t.Errorf("MirroredLocationFor() = %q, want %q", got, want)
		}
	})

	t.Run("nil mapping returns original", func(t *testing.T) {
		got := MirroredLocationFor(nil, "quay.io/test:v1")
		if got != "quay.io/test:v1" {
			t.Errorf("MirroredLocationFor(nil) = %q, want original", got)
		}
	})
}

func TestLoadMirroredImagesFromEnv(t *testing.T) {
	t.Run("unset env returns nil", func(t *testing.T) {
		os.Unsetenv(MirroredImagesEnvVar)
		mapping, err := LoadMirroredImagesFromEnv()
		if err != nil {
			t.Fatalf("LoadMirroredImagesFromEnv() error = %v", err)
		}
		if mapping != nil {
			t.Errorf("LoadMirroredImagesFromEnv() = %v, want nil", mapping)
		}
	})

	t.Run("set env loads file", func(t *testing.T) {
		original := map[string]string{
			"quay.io/test:v1": "private.com/repo:mirrored-v1",
		}
		path, err := WriteMirroredImagesFile(original)
		if err != nil {
			t.Fatalf("WriteMirroredImagesFile() error = %v", err)
		}
		defer os.Remove(path)

		t.Setenv(MirroredImagesEnvVar, path)

		mapping, err := LoadMirroredImagesFromEnv()
		if err != nil {
			t.Fatalf("LoadMirroredImagesFromEnv() error = %v", err)
		}
		if mapping["quay.io/test:v1"] != "private.com/repo:mirrored-v1" {
			t.Errorf("unexpected mapping: %v", mapping)
		}
	})
}
