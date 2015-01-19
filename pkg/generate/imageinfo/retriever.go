package imageinfo

import (
	"github.com/GoogleCloudPlatform/kubernetes/pkg/labels"
	"github.com/fsouza/go-dockerclient"

	"github.com/openshift/origin/pkg/generate/errors"
	image "github.com/openshift/origin/pkg/image/api"
)

// Retriever is an interface for an image information retriever
type Retriever interface {
	Retrieve(name string) (*docker.Image, error)
}

type retriever struct {
	streamLister imageStreamLister
	imageGetter  imageGetter
	dockerClient dockerClient
}

type dockerClient interface {
	InspectImage(name string) (*docker.Image, error)
}

type imageStreamLister interface {
	List(label, field labels.Selector) (*image.ImageRepositoryList, error)
}

type imageGetter interface {
	Get(name string) (*image.Image, error)
}

// NewRetriever creates a new retriever using the passed in interfaces for OpenShift image streams as well as Docker client
func NewRetriever(streamLister imageStreamLister, imageGetter imageGetter, dockerClient dockerClient) Retriever {
	return &retriever{
		streamLister: streamLister,
		imageGetter:  imageGetter,
		dockerClient: dockerClient,
	}
}

// Retrieve fetches image metadata for a given image name. It will first
// try the OpenShift server and then the local Docker daemon.
func (r *retriever) Retrieve(name string) (*docker.Image, error) {
	// TODO: implement a way to discover images on the server via REST client
	// Try finding the image on the openshift server first
	if r.streamLister != nil {
		streamList, err := r.streamLister.List(labels.Everything(), labels.Everything())
		if err == nil {
			for _, imageStream := range streamList.Items {
				_, ns, nm, _, err := image.SplitDockerPullSpec(imageStream.DockerImageRepository)
				if err != nil {
					continue
				}
				if name == ns+"/"+nm {
					for _, imageName := range imageStream.Tags {
						img, err := r.imageGetter.Get(imageName)
						if err != nil {
							continue
						}
						return &img.DockerImageMetadata, nil
					}
				}
			}
		}
	}

	// TODO: Use the Docker registry API to retrieve image information if possible

	// If that doesn't work try Docker if present
	if r.dockerClient != nil {
		return r.dockerClient.InspectImage(name)
	}

	return nil, errors.ImageNotFound
}
