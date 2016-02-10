package imagestreamimage

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/digest"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// REST implements the RESTStorage interface in terms of an image registry and
// image stream registry. It only supports the Get method and is used
// to retrieve an image by id, scoped to an ImageStream. REST ensures
// that the requested image belongs to the specified ImageStream.
type REST struct {
	imageRegistry       image.Registry
	imageStreamRegistry imagestream.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry) *REST {
	return &REST{imageRegistry, imageStreamRegistry}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.ImageStreamImage{}
}

// ParseNameAndID splits a string into its name component and ID component, and returns an error
// if the string is not in the right form.
func ParseNameAndID(input string) (name string, id string, err error) {
	segments := strings.Split(input, "@")
	switch len(segments) {
	case 2:
		name = segments[0]
		id = segments[1]
		if len(name) == 0 || len(id) == 0 {
			err = errors.NewBadRequest("ImageStreamImages must be retrieved with <name>@<id>")
		}
	default:
		err = errors.NewBadRequest("ImageStreamImages must be retrieved with <name>@<id>")
	}
	return
}

// Get retrieves an image by ID that has previously been tagged into an image stream.
// `id` is of the form <repo name>@<image id>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, imageID, err := ParseNameAndID(id)
	if err != nil {
		return nil, err
	}

	repo, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	if repo.Status.Tags == nil {
		return nil, errors.NewNotFound(api.Resource("imagestreamimage"), imageID)
	}

	event, err := api.ResolveImageID(repo, imageID)
	if err != nil {
		return nil, err
	}

	imageName := event.Image
	image, err := r.imageRegistry.GetImage(ctx, imageName)
	if err != nil {
		return nil, err
	}
	if err := api.ImageWithMetadata(image); err != nil {
		return nil, err
	}
	image.DockerImageManifest = ""

	if d, err := digest.ParseDigest(imageName); err == nil {
		imageName = d.Hex()
	}
	if len(imageName) > 7 {
		imageName = imageName[:7]
	}

	isi := api.ImageStreamImage{
		ObjectMeta: kapi.ObjectMeta{
			Namespace: kapi.NamespaceValue(ctx),
			Name:      fmt.Sprintf("%s@%s", name, imageName),
		},
		Image: *image,
	}

	return &isi, nil
}
