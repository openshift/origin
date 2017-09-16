package v1_test

import (
	"reflect"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/diff"
	kapi "k8s.io/kubernetes/pkg/api"

	"github.com/openshift/origin/pkg/api/apihelpers/apitesting"
	newer "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apis/image/docker10"
	"github.com/openshift/origin/pkg/image/apis/image/dockerpre012"
	imageapiv1 "github.com/openshift/origin/pkg/image/apis/image/v1"

	// some side-effect of this import is causing TestRoundTripVersionedObject to pass.  I don't see it.
	_ "github.com/openshift/origin/pkg/image/apis/image/install"
)

func TestRoundTripVersionedObject(t *testing.T) {
	scheme := runtime.NewScheme()
	docker10.AddToSchemeInCoreGroup(scheme)
	dockerpre012.AddToSchemeInCoreGroup(scheme)
	newer.AddToSchemeInCoreGroup(scheme)
	docker10.AddToScheme(scheme)
	dockerpre012.AddToScheme(scheme)
	imageapiv1.AddToSchemeInCoreGroup(scheme)
	newer.AddToScheme(scheme)
	imageapiv1.AddToScheme(scheme)
	codecs := serializer.NewCodecFactory(scheme)

	d := &newer.DockerImage{
		Config: &newer.DockerConfig{
			Env: []string{"A=1", "B=2"},
		},
	}
	i := &newer.Image{
		ObjectMeta: metav1.ObjectMeta{Name: "foo"},

		DockerImageLayers:    []newer.ImageLayer{{Name: "foo", LayerSize: 10}},
		DockerImageMetadata:  *d,
		DockerImageReference: "foo/bar/baz",
	}

	data, err := runtime.Encode(codecs.LegacyCodec(imageapiv1.SchemeGroupVersion), i)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	obj, err := runtime.Decode(codecs.UniversalDecoder(), data)
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

func TestFieldSelectors(t *testing.T) {
	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{imageapiv1.LegacySchemeBuilder.AddToScheme, newer.LegacySchemeBuilder.AddToScheme},
		Kind:          imageapiv1.LegacySchemeGroupVersion.WithKind("ImageStream"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"name", "spec.dockerImageRepository", "status.dockerImageRepository"},
		FieldKeyEvaluatorFn:      newer.ImageStreamSelector,
	}.Check(t)

	apitesting.FieldKeyCheck{
		SchemeBuilder: []func(*runtime.Scheme) error{imageapiv1.SchemeBuilder.AddToScheme, newer.SchemeBuilder.AddToScheme},
		Kind:          imageapiv1.SchemeGroupVersion.WithKind("ImageStream"),
		// Ensure previously supported labels have conversions. DO NOT REMOVE THINGS FROM THIS LIST
		AllowedExternalFieldKeys: []string{"spec.dockerImageRepository", "status.dockerImageRepository"},
		FieldKeyEvaluatorFn:      newer.ImageStreamSelector,
	}.Check(t)
}

func TestImageImportSpecDefaulting(t *testing.T) {
	scheme := runtime.NewScheme()
	codecs := serializer.NewCodecFactory(scheme)
	imageapiv1.LegacySchemeBuilder.AddToScheme(scheme)
	imageapiv1.SchemeBuilder.AddToScheme(scheme)
	newer.LegacySchemeBuilder.AddToScheme(scheme)
	newer.SchemeBuilder.AddToScheme(scheme)

	i := &newer.ImageStreamImport{
		Spec: newer.ImageStreamImportSpec{
			Images: []newer.ImageImportSpec{
				{From: kapi.ObjectReference{Name: "something:other"}},
			},
		},
	}
	data, err := runtime.Encode(codecs.LegacyCodec(imageapiv1.SchemeGroupVersion), i)
	if err != nil {
		t.Fatal(err)
	}
	obj, err := runtime.Decode(codecs.UniversalDecoder(), data)
	if err != nil {
		t.Fatal(err)
	}
	isi := obj.(*newer.ImageStreamImport)
	if isi.Spec.Images[0].To == nil || isi.Spec.Images[0].To.Name != "other" {
		t.Errorf("unexpected round trip: %#v", isi)
	}
}
