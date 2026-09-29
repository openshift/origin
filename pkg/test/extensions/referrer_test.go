package extensions

import (
	"testing"

	"github.com/opencontainers/go-digest"
	ocispec "github.com/opencontainers/image-spec/specs-go/v1"
)

func TestExtensionLayer(t *testing.T) {
	subject := ocispec.Descriptor{Digest: digest.FromString("component image")}
	other := ocispec.Descriptor{Digest: digest.FromString("other image")}
	layer := ocispec.Descriptor{
		MediaType:   testExtensionLayerType,
		Annotations: map[string]string{ociTitleAnnotation: "operator-tests-ext.gz"},
	}
	base := func() ocispec.Manifest {
		return ocispec.Manifest{
			Subject:      &subject,
			ArtifactType: testExtensionArtifactType,
			Layers:       []ocispec.Descriptor{layer},
		}
	}
	tests := []struct {
		name   string
		mutate func(*ocispec.Manifest)
		want   bool
	}{
		{name: "matching artifact", want: true},
		{name: "wrong subject", mutate: func(m *ocispec.Manifest) { m.Subject = &other }},
		{name: "missing subject", mutate: func(m *ocispec.Manifest) { m.Subject = nil }},
		{name: "wrong artifact type", mutate: func(m *ocispec.Manifest) { m.ArtifactType = "application/example" }},
		{name: "wrong layer type", mutate: func(m *ocispec.Manifest) { m.Layers[0].MediaType = "application/octet-stream" }},
		{name: "wrong title", mutate: func(m *ocispec.Manifest) { m.Layers[0].Annotations = map[string]string{ociTitleAnnotation: "other.gz"} }},
		{name: "multiple layers", mutate: func(m *ocispec.Manifest) { m.Layers = append(m.Layers, layer) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := base()
			if tt.mutate != nil {
				tt.mutate(&manifest)
			}
			_, ok := extensionLayer(manifest, subject, "operator-tests-ext.gz")
			if ok != tt.want {
				t.Fatalf("extensionLayer() matched = %v, want %v", ok, tt.want)
			}
		})
	}
}
