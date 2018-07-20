package legacy

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/apis/core"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	internal "github.com/openshift/origin/pkg/image/apis/image"
)

func TestImageRoundTripVersionedObject(t *testing.T) {
	scheme := runtime.NewScheme()
	InstallInternalLegacyImage(scheme)
	codecs := serializer.NewCodecFactory(scheme)

	d := &internal.DockerImage{
		Config: &internal.DockerConfig{
			Env: []string{"A=1", "B=2"},
		},
	}
	i := &internal.Image{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},

		DockerImageLayers:    []internal.ImageLayer{{Name: "foo", LayerSize: 10}},
		DockerImageMetadata:  *d,
		DockerImageReference: "foo/bar/baz",
	}

	data, err := runtime.Encode(codecs.LegacyCodec(GroupVersion), i)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	image := obj.(*internal.Image)
	if image.DockerImageMetadataVersion != "1.0" {
		t.Errorf("did not default to correct metadata version: %#v", image)
	}
	image.DockerImageMetadataVersion = ""
	if !reflect.DeepEqual(i, image) {
		t.Errorf("unable to round trip object: %s", diff.ObjectDiff(i, image))
	}
}

func TestImageFieldSelectors(t *testing.T) {
	install := func(scheme *runtime.Scheme) error {
		InstallInternalLegacyImage(scheme)
		return nil
	}

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{install},
		Kind:          GroupVersion.WithKind("ImageStream"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"name", "spec.dockerImageRepository", "status.dockerImageRepository"},
		FieldKeyEvaluatorFn:      internal.ImageStreamSelector,
	}.Check(t)
}

func TestImageImportSpecDefaulting(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	InstallInternalLegacyImage(scheme)

	i := &internal.ImageStreamImport{
		Spec: internal.ImageStreamImportSpec{
			Images: []internal.ImageImportSpec{
				{From: kapi.ObjectReference{Name: "something:other"}},
			},
		},
	}
	data, err := runtime.Encode(codecs.LegacyCodec(GroupVersion), i)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	isi := obj.(*internal.ImageStreamImport)
	if isi.Spec.Images[0].To == nil || isi.Spec.Images[0].To.Name != "other" {
		t.Errorf("unexpected round trip: %#v", isi)
	}
}
