package v1

import (
	"testing"

	"k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/conversion"
)

func TestAPItoV1VolumeSourceConversion(t *testing.T) {
	c := conversion.NewConverter()
	c.Debug = t

	if err := c.RegisterConversionFunc(Convert_api_VolumeSource_To_v1_VolumeSource); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	in := api.VolumeSource{
		DownwardAPI: &api.DownwardAPIVolumeSource{
			Items: []api.DownwardAPIVolumeFile{
				{
					Path: "./test/api-to-v1/conversion",
				},
			},
		},
	}
	out := VolumeSource{}

	if err := c.Convert(&in, &out, 0, nil); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if e, a := in.DownwardAPI.Items[0].Path, out.Metadata.Items[0].Name; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := in.DownwardAPI.Items[0].Path, out.DownwardAPI.Items[0].Path; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}

func TestV1toAPIVolumeSourceConversion(t *testing.T) {
	c := conversion.NewConverter()
	c.Debug = t

	if err := c.RegisterConversionFunc(Convert_v1_VolumeSource_To_api_VolumeSource); err != nil {
		t.Fatalf("unexpected error %v", err)
	}

	in := VolumeSource{
		DownwardAPI: &DownwardAPIVolumeSource{
			Items: []DownwardAPIVolumeFile{
				{
					Path: "./test/v1-to-api/conversion",
				},
			},
		},
		Metadata: &MetadataVolumeSource{
			Items: []MetadataFile{
				{
					Name: "./test/v1-to-api/conversion",
				},
			},
		},
	}
	out := api.VolumeSource{}

	if err := c.Convert(&in, &out, 0, nil); err != nil {
		t.Fatalf("unexpected error %v", err)
	}
	if e, a := in.Metadata.Items[0].Name, out.DownwardAPI.Items[0].Path; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
	if e, a := in.DownwardAPI.Items[0].Path, out.DownwardAPI.Items[0].Path; e != a {
		t.Errorf("expected %v, got %v", e, a)
	}
}
