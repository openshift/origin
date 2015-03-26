package imagestreamimage

import (
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"
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
	return &api.Image{}
}

// nameAndID splits a string into its name component and ID component, and returns an error
// if the string is not in the right form.
func nameAndID(input string) (name string, id string, err error) {
	segments := strings.Split(input, "@")
	switch len(segments) {
	case 2:
		name = segments[0]
		id = segments[1]
		if len(name) == 0 || len(id) == 0 {
			err = errors.NewBadRequest("imageStreamImages must be retrieved with <name>@<id>")
		}
	default:
		err = errors.NewBadRequest("imageStreamImages must be retrieved with <name>@<id>")
	}
	return
}

// Get retrieves an image by ID that has previously been tagged into an image stream.
// `id` is of the form <repo name>@<image id>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, imageID, err := nameAndID(id)
	if err != nil {
		return nil, err
	}

	repo, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	if repo.Status.Tags == nil {
		return nil, errors.NewNotFound("imageStreamImage", imageID)
	}

	for _, history := range repo.Status.Tags {
		for _, tagging := range history.Items {
			if tagging.Image == imageID {
				image, err := r.imageRegistry.GetImage(ctx, imageID)
				if err != nil {
					return nil, err
				}
				return api.ImageWithMetadata(*image)
			}
		}
	}

	return nil, errors.NewNotFound("imageStreamImage", imageID)
}
