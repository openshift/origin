package imagestreamtag

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagestream"
)

// REST implements the RESTStorage interface for ImageStreamTag
// It only supports the Get method and is used to simplify retrieving an Image by tag from an ImageStream
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
	return &api.ImageStreamTag{}
}

// nameAndTag splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func nameAndTag(id string) (name string, tag string, err error) {
	segments := strings.Split(id, ":")
	switch len(segments) {
	case 2:
		name = segments[0]
		tag = segments[1]
		if len(name) == 0 || len(tag) == 0 {
			err = errors.NewBadRequest("ImageStreamTags must be retrieved with <name>:<tag>")
		}
	default:
		err = errors.NewBadRequest("ImageStreamTags must be retrieved with <name>:<tag>")
	}
	return
}

// Get retrieves an image that has been tagged by stream and tag. `id` is of the format
// <stream name>:<tag>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	stream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	event := api.LatestTaggedImage(stream, tag)
	if event == nil || len(event.Image) == 0 {
		return nil, errors.NewNotFound("imageStreamTag", id)
	}

	image, err := r.imageRegistry.GetImage(ctx, event.Image)
	if err != nil {
		return nil, err
	}

	// if the stream has Spec.Tags[tag].Annotations[k] = v, copy it to the image's annotations
	if stream.Spec.Tags != nil {
		if tagRef, ok := stream.Spec.Tags[tag]; ok {
			if image.Annotations == nil {
				image.Annotations = make(map[string]string)
			}
			for k, v := range tagRef.Annotations {
				image.Annotations[k] = v
			}
		}
	}

	imageWithMetadata, err := api.ImageWithMetadata(*image)
	if err != nil {
		return nil, err
	}

	ist := api.ImageStreamTag{
		Image:     *imageWithMetadata,
		ImageName: imageWithMetadata.Name,
	}
	ist.Namespace = kapi.NamespaceValue(ctx)
	ist.Name = id
	return &ist, nil
}

// Delete removes a tag from a stream. `id` is of the format <stream name>:<tag>.
// The associated image that the tag points to is *not* deleted.
// The tag history remains intact and is not deleted.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	stream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	if stream.Spec.Tags == nil {
		return nil, errors.NewNotFound("imageStreamTag", tag)
	}

	_, ok := stream.Spec.Tags[tag]
	if !ok {
		return nil, errors.NewNotFound("imageStreamTag", tag)
	}

	delete(stream.Spec.Tags, tag)

	_, err = r.imageStreamRegistry.UpdateImageStream(ctx, stream)
	if err != nil {
		return nil, fmt.Errorf("Error removing tag from image stream: %s", err)
	}

	return &kapi.Status{Status: kapi.StatusSuccess}, nil
}
