package v1_test

import (
	"reflect"
	"testing"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/util"

	_ "github.com/openshift/origin/pkg/api/latest"
	newer "github.com/openshift/origin/pkg/image/api"
)

var Convert = kapi.Scheme.Convert

func TestRoundTripVersionedObject(t *testing.T) {
	d := &newer.DockerImage{
		Config: &newer.DockerConfig{
			Env: []string{"A=1", "B=2"},
		},
	}
	i := &newer.Image{
		ObjectMeta: kapi.ObjectMeta{Name: "foo"},

		DockerImageMetadata:  *d,
		DockerImageReference: "foo/bar/baz",
	}

	data, err := kapi.Scheme.EncodeToVersion(i, "v1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := kapi.Scheme.Decode(data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image := obj.(*newer.Image)
	if image.DockerImageMetadataVersion != "1.0" {
		t.Errorf("did not default to correct metadata version: %#v", image)
	}
	image.DockerImageMetadataVersion = ""
	if !reflect.DeepEqual(i, image) {
		t.Errorf("unable to round trip object: %s", util.ObjectDiff(i, image))
	}
}
