package v1beta3_test

import (
	"reflect"
	"testing"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/runtime"
	"k8s.io/kubernetes/pkg/util/diff"

	newer "github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/api/v1beta3"

	_ "github.com/openshift/origin/pkg/api/install"
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

	data, err := runtime.Encode(kapi.Codecs.LegacyCodec(v1beta3.SchemeGroupVersion), i)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := runtime.Decode(kapi.Codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image := obj.(*newer.Image)
	if image.DockerImageMetadataVersion != "1.0" {
		t.Errorf("did not default to correct metadata version: %#v", image)
	}
	image.DockerImageMetadataVersion = ""
	if !reflect.DeepEqual(i, image) {
		t.Errorf("unable to round trip object: %s", diff.ObjectDiff(i, image))
	}
}
