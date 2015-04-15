package app

import (
	"testing"

	image "github.com/openshift/origin/pkg/image/api"
)

func TestFromName(t *testing.T) {
	g := NewImageRefGenerator()
	imageRef, err := g.FromName("some.registry.com:1234/namespace1/name:tag1234")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if imageRef.Registry != "some.registry.com:1234" {
		t.Errorf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "namespace1" {
		t.Errorf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "name" {
		t.Errorf("Unexpected name: %s", imageRef.Name)
	}
	if imageRef.Tag != "tag1234" {
		t.Errorf("Unexpected tag: %s", imageRef.Tag)
	}
}

func TestFromStream(t *testing.T) {
	g := NewImageRefGenerator()
	repo := image.ImageStream{
		Status: image.ImageStreamStatus{
			DockerImageRepository: "my.registry:5000/test/image",
		},
	}
	imageRef, err := g.FromStream(&repo, "tag1234")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if imageRef.Registry != "my.registry:5000" {
		t.Errorf("Unexpected registry: %s", imageRef.Registry)
	}
	if imageRef.Namespace != "test" {
		t.Errorf("Unexpected namespace: %s", imageRef.Namespace)
	}
	if imageRef.Name != "image" {
		t.Errorf("Unenxpected name: %s", imageRef.Name)
	}
}
