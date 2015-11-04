package imagestreamtag

import (
	"fmt"
	"strings"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/runtime"

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
	return &REST{imageRegistry: imageRegistry, imageStreamRegistry: imageStreamRegistry}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.ImageStreamTag{}
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return &api.ImageStreamTagList{}
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
			err = kapierrors.NewBadRequest("ImageStreamTags must be retrieved with <name>:<tag>")
		}
	default:
		err = kapierrors.NewBadRequest("ImageStreamTags must be retrieved with <name>:<tag>")
	}
	return
}

func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	imageStreams, err := r.imageStreamRegistry.ListImageStreams(ctx, labels.Everything())
	if err != nil {
		return nil, err
	}

	matcher := MatchImageStreamTag(label, field)

	list := &api.ImageStreamTagList{}
	for _, currIS := range imageStreams.Items {
		for currTag := range currIS.Status.Tags {
			istag, err := newISTag(currTag, &currIS, nil)
			if err != nil {
				return nil, err
			}
			matches, err := matcher.Matches(istag)
			if err != nil {
				return nil, err
			}

			if matches {
				list.Items = append(list.Items, *istag)
			}
		}
	}

	return list, nil
}

// Get retrieves an image that has been tagged by stream and tag. `id` is of the format <stream name>:<tag>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	imageStream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		return nil, err
	}

	image, err := r.imageFor(ctx, tag, imageStream)
	if err != nil {
		return nil, err
	}

	return newISTag(tag, imageStream, image)
}

func (r *REST) Update(ctx kapi.Context, obj runtime.Object) (runtime.Object, bool, error) {
	istag, ok := obj.(*api.ImageStreamTag)
	if !ok {
		return nil, false, kapierrors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamTag: %#v", obj))
	}

	old, err := r.Get(ctx, istag.Name)
	if err != nil {
		return nil, false, err
	}

	if err := rest.BeforeUpdate(Strategy, ctx, obj, old); err != nil {
		return nil, false, err
	}

	// we only allow updates of annotations, so lets find the correct image stream and update it.
	name, tag, err := nameAndTag(istag.Name)
	if err != nil {
		return nil, false, err
	}

	imageStream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if imageStream.Spec.Tags == nil {
		imageStream.Spec.Tags = map[string]api.TagReference{}
	}
	tagRef := imageStream.Spec.Tags[tag]
	tagRef.Annotations = istag.Annotations
	imageStream.Spec.Tags[tag] = tagRef

	newImageStream, err := r.imageStreamRegistry.UpdateImageStream(ctx, imageStream)
	if err != nil {
		return nil, false, err
	}

	image, err := r.imageFor(ctx, tag, newImageStream)
	if err != nil {
		return nil, false, err
	}

	newISTag, err := newISTag(tag, newImageStream, image)
	return newISTag, false, err
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

	notFound := true

	// Try to delete the status tag
	if _, ok := stream.Status.Tags[tag]; ok {
		delete(stream.Status.Tags, tag)
		notFound = false
	}

	// Try to delete the spec tag
	if _, ok := stream.Spec.Tags[tag]; ok {
		delete(stream.Spec.Tags, tag)
		notFound = false
	}

	if notFound {
		return nil, kapierrors.NewNotFound("imageStreamTag", tag)
	}

	if _, err = r.imageStreamRegistry.UpdateImageStream(ctx, stream); err != nil {
		return nil, fmt.Errorf("cannot remove tag from image stream: %v", err)
	}

	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

// imageFor retrieves the most recent image for a tag in a given imageStreem.
func (r *REST) imageFor(ctx kapi.Context, tag string, imageStream *api.ImageStream) (*api.Image, error) {
	event := api.LatestTaggedImage(imageStream, tag)
	if event == nil || len(event.Image) == 0 {
		return nil, kapierrors.NewNotFound("imageStreamTag", api.JoinImageStreamTag(imageStream.Name, tag))
	}

	return r.imageRegistry.GetImage(ctx, event.Image)
}

func newISTag(tag string, imageStream *api.ImageStream, image *api.Image) (*api.ImageStreamTag, error) {
	istagName := api.JoinImageStreamTag(imageStream.Name, tag)

	event := api.LatestTaggedImage(imageStream, tag)
	if event == nil || len(event.Image) == 0 {
		return nil, kapierrors.NewNotFound("imageStreamTag", istagName)
	}

	ist := &api.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{
			Namespace:         imageStream.Namespace,
			Name:              istagName,
			CreationTimestamp: event.Created,
			Annotations:       map[string]string{},
			ResourceVersion:   imageStream.ResourceVersion,
		},
	}

	// if the imageStream has Spec.Tags[tag].Annotations[k] = v, copy it to the image's annotations
	// and add them to the istag's annotations
	if imageStream.Spec.Tags != nil {
		if tagRef, ok := imageStream.Spec.Tags[tag]; ok {
			if image != nil && image.Annotations == nil {
				image.Annotations = make(map[string]string)
			}
			for k, v := range tagRef.Annotations {
				ist.Annotations[k] = v
				if image != nil {
					image.Annotations[k] = v
				}
			}
		}
	}

	if image != nil {
		imageWithMetadata, err := api.ImageWithMetadata(*image)
		if err != nil {
			return nil, err
		}
		ist.Image = *imageWithMetadata
	} else {
		ist.Image = api.Image{}
		ist.Image.Name = event.Image
	}

	// Replace the DockerImageReference with the value from event, which contains
	// real value from status. This should fix the problem for v1 registries,
	// where mutliple tags point to a single id and only the first image's metadata
	// is saved. This in turn will always return the pull spec from the first
	// imported image, which might be different than the requested tag.
	ist.Image.DockerImageReference = event.DockerImageReference

	return ist, nil
}
