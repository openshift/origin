package imageinfo

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/fsouza/go-dockerclient"

	image "github.com/openshift/origin/pkg/image/api"
)

func TestRetrieveFromOpenShift(t *testing.T) {
	imageMetadata := docker.Image{
		ID:   "image2",
		Size: 2048,
	}
	client := fakeOSClient{
		repositories: []image.ImageRepository{
			{
				DockerImageRepository: "test/repository1",
			},
			{
				DockerImageRepository: "test/repository2",
				Tags: map[string]string{
					"a_test_image_tag": "image1",
					"a_second_tag":     "image2",
				},
			},
		},
		images: map[string]image.Image{
			"image2": {
				DockerImageMetadata: imageMetadata,
			},
		},
	}
	r := NewRetriever(&client, &client, &fakeDockerClient{})
	result, err := r.Retrieve("test/repository2")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(*result, imageMetadata) {
		t.Errorf("Unexpected result: %#v", result)
	}
}

func TestRetrieveFromLocalDocker(t *testing.T) {
	osClient := &fakeOSClient{}
	image1 := docker.Image{
		ID:   "image1",
		Size: 1024,
	}
	dockerClient := &fakeDockerClient{
		images: map[string]docker.Image{
			"test/image1": image1,
		},
	}
	r := NewRetriever(osClient, osClient, dockerClient)
	result, err := r.Retrieve("test/image1")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
	if !reflect.DeepEqual(*result, image1) {
		t.Errorf("Unexpected result: %#v", result)
	}
}

type fakeOSClient struct {
	repositories []image.ImageRepository
	images       map[string]image.Image
}

func (c *fakeOSClient) List(label, field labels.Selector) (*image.ImageRepositoryList, error) {
	return &image.ImageRepositoryList{
		Items: c.repositories,
	}, nil
}

func (c *fakeOSClient) Get(name string) (*image.Image, error) {
	img, ok := c.images[name]
	if ok {
		return &img, nil
	}
	return nil, fmt.Errorf("Not found")
}

type fakeDockerClient struct {
	images map[string]docker.Image
}

func (d *fakeDockerClient) InspectImage(name string) (*docker.Image, error) {
	img, ok := d.images[name]
	if ok {
		return &img, nil
	}
	return nil, fmt.Errorf("Not found")
}
