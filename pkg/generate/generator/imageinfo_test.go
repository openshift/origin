package generator

import (
	"testing"

	"github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/generate/app"
)

func TestFromImageRef(t *testing.T) {
	img := &docker.Image{
		ID: "test/image",
	}
	g := NewImageInfoGenerator(&fakeRetriever{img})

	imageRef := app.ImageRef{Namespace: "test", Name: "image"}
	imageInfo := g.FromImageRef(imageRef)
	if imageInfo.Info != img {
		t.Errorf("Unexpected image info returned.")
	}
}

func TestFromImageRefs(t *testing.T) {
	img := &docker.Image{
		ID: "test/image",
	}
	g := NewImageInfoGenerator(&fakeRetriever{img})

	imageRefs := []app.ImageRef{
		{Namespace: "test", Name: "image1"},
		{Namespace: "test", Name: "image2"},
		{Namespace: "test", Name: "image3"},
	}
	imageInfos := g.FromImageRefs(imageRefs)
	if len(imageInfos) != 3 {
		t.Errorf("Unexpected number of imagerefs returned")
	}
	if imageInfos[0].Info != img {
		t.Errorf("Unexpected image info returned.")
	}
}

type fakeRetriever struct {
	image *docker.Image
}

func (r *fakeRetriever) Retrieve(name string) (*docker.Image, error) {
	return r.image, nil
}
