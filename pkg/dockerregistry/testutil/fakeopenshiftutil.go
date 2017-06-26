package testutil

import (
	"fmt"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// AddImageStream creates a new image stream with annotations.
func AddImageStream(t *testing.T, fos *FakeOpenShift, namespace, name string, annotations map[string]string) *imageapi.ImageStream {
	is := &imageapi.ImageStream{}
	is.Name = name
	is.Annotations = annotations

	is, err := fos.CreateImageStream(namespace, is)
	if err != nil {
		t.Fatal(err)
	}
	return is
}

// AddUntaggedImage creates image in fos.
func AddUntaggedImage(t *testing.T, fos *FakeOpenShift, image *imageapi.Image) {
	_, err := fos.CreateImage(image)
	if err != nil {
		t.Fatal(err)
	}
}

// AddImage tags image into the image stream namespace/name.
func AddImage(t *testing.T, fos *FakeOpenShift, image *imageapi.Image, namespace, name, tag string) {
	_, err := fos.CreateImageStreamMapping(namespace, &imageapi.ImageStreamMapping{
		ObjectMeta: metav1.ObjectMeta{
			Namespace: namespace,
			Name:      name,
		},
		Image: *image,
		Tag:   tag,
	})
	if err != nil {
		t.Fatal(err)
	}
}

// AddRandomImage creates a new image with a random content and tags it into
// the image stream namespace/name. If the image stream doesn't exists, it
// will be created.
func AddRandomImage(t *testing.T, fos *FakeOpenShift, namespace, name, tag string) *imageapi.Image {
	image, err := CreateRandomImage(namespace, name)
	if err != nil {
		t.Fatal(err)
	}

	_, err = fos.GetImageStream(namespace, name)
	if err != nil {
		AddImageStream(t, fos, namespace, name, map[string]string{
			imageapi.InsecureRepositoryAnnotation: "true",
		})
	}

	AddImage(t, fos, image, namespace, name, tag)

	return image
}

// AddImageStreamTag creates an image stream tag.
func AddImageStreamTag(t *testing.T, fos *FakeOpenShift, image *imageapi.Image, namespace, name string, tag *imageapi.TagReference) *imageapi.ImageStreamTag {
	istag, err := fos.CreateImageStreamTag(namespace, &imageapi.ImageStreamTag{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("%s:%s", name, tag.Name),
		},
		Tag:   tag,
		Image: *image,
	})
	if err != nil {
		t.Fatal(err)
	}
	return istag
}
