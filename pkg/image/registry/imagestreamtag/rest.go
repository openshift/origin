package imagestreamtag

import (
	"fmt"

	kapi "k8s.io/kubernetes/pkg/api"
	kapierrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/api/rest"
	"k8s.io/kubernetes/pkg/api/unversioned"
	"k8s.io/kubernetes/pkg/runtime"

	oapi "github.com/openshift/origin/pkg/api"
	imageapi "github.com/openshift/origin/pkg/image/api"
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

var _ rest.Creater = &REST{}
var _ rest.Lister = &REST{}
var _ rest.Getter = &REST{}
var _ rest.Deleter = &REST{}
var _ rest.Updater = &REST{}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &imageapi.ImageStreamTag{}
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return &imageapi.ImageStreamTagList{}
}

// nameAndTag splits a string into its name component and tag component, and returns an error
// if the string is not in the right form.
func nameAndTag(id string) (name string, tag string, err error) {
	name, tag, err = imageapi.ParseImageStreamTagName(id)
	if err != nil {
		err = kapierrors.NewBadRequest("ImageStreamTags must be retrieved with <name>:<tag>")
	}
	return
}

func (r *REST) List(ctx kapi.Context, options *kapi.ListOptions) (runtime.Object, error) {
	imageStreams, err := r.imageStreamRegistry.ListImageStreams(ctx, options)
	if err != nil {
		return nil, err
	}

	matcher := MatchImageStreamTag(oapi.ListOptionsToSelectors(options))

	list := &imageapi.ImageStreamTagList{}
	for _, currIS := range imageStreams.Items {
		for currTag := range currIS.Status.Tags {
			istag, err := newISTag(currTag, &currIS, nil, false)
			if err != nil {
				if kapierrors.IsNotFound(err) {
					continue
				}
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

	return newISTag(tag, imageStream, image, false)
}

func (r *REST) Create(ctx kapi.Context, obj runtime.Object) (runtime.Object, error) {
	istag, ok := obj.(*imageapi.ImageStreamTag)
	if !ok {
		return nil, kapierrors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamTag: %#v", obj))
	}
	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}
	namespace, ok := kapi.NamespaceFrom(ctx)
	if !ok {
		return nil, kapierrors.NewBadRequest("a namespace must be specified to import images")
	}

	imageStreamName, imageTag, ok := imageapi.SplitImageStreamTag(istag.Name)
	if !ok {
		return nil, fmt.Errorf("%q must be of the form <stream_name>:<tag>", istag.Name)
	}

	target, err := r.imageStreamRegistry.GetImageStream(ctx, imageStreamName)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, err
		}

		// try to create the target if it doesn't exist
		target = &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Name:      imageStreamName,
				Namespace: namespace,
			},
		}
	}

	if target.Spec.Tags == nil {
		target.Spec.Tags = make(map[string]imageapi.TagReference)
	}

	// The user wants to symlink a tag.
	_, exists := target.Spec.Tags[imageTag]
	if exists {
		return nil, kapierrors.NewAlreadyExists(imageapi.Resource("imagestreamtag"), istag.Name)
	}
	target.Spec.Tags[imageTag] = *istag.Tag

	// Check the stream creation timestamp and make sure we will not
	// create a new image stream while deleting.
	if target.CreationTimestamp.IsZero() {
		_, err = r.imageStreamRegistry.CreateImageStream(ctx, target)
	} else {
		_, err = r.imageStreamRegistry.UpdateImageStream(ctx, target)
	}
	if err != nil {
		return nil, err
	}

	return istag, nil
}

func (r *REST) Update(ctx kapi.Context, tagName string, objInfo rest.UpdatedObjectInfo) (runtime.Object, bool, error) {
	name, tag, err := nameAndTag(tagName)
	if err != nil {
		return nil, false, err
	}

	create := false
	imageStream, err := r.imageStreamRegistry.GetImageStream(ctx, name)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, false, err
		}
		namespace, ok := kapi.NamespaceFrom(ctx)
		if !ok {
			return nil, false, kapierrors.NewBadRequest("namespace is required on ImageStreamTags")
		}
		imageStream = &imageapi.ImageStream{
			ObjectMeta: kapi.ObjectMeta{
				Namespace: namespace,
				Name:      name,
			},
		}
		kapi.FillObjectMetaSystemFields(ctx, &imageStream.ObjectMeta)
		create = true
	}

	// create the synthetic old istag
	old, err := newISTag(tag, imageStream, nil, true)
	if err != nil {
		return nil, false, err
	}

	obj, err := objInfo.UpdatedObject(ctx, old)
	if err != nil {
		return nil, false, err
	}

	istag, ok := obj.(*imageapi.ImageStreamTag)
	if !ok {
		return nil, false, kapierrors.NewBadRequest(fmt.Sprintf("obj is not an ImageStreamTag: %#v", obj))
	}

	// check for conflict
	switch {
	case len(istag.ResourceVersion) == 0:
		// should disallow blind PUT, but this was previously supported
		istag.ResourceVersion = imageStream.ResourceVersion
	case len(imageStream.ResourceVersion) == 0:
		// image stream did not exist, cannot update
		return nil, false, kapierrors.NewNotFound(imageapi.Resource("imagestreamtags"), tagName)
	case imageStream.ResourceVersion != istag.ResourceVersion:
		// conflicting input and output
		return nil, false, kapierrors.NewConflict(imageapi.Resource("imagestreamtags"), istag.Name, fmt.Errorf("another caller has updated the resource version to %s", imageStream.ResourceVersion))
	}

	if create {
		if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
			return nil, false, err
		}
	} else {
		if err := rest.BeforeUpdate(Strategy, ctx, obj, old); err != nil {
			return nil, false, err
		}
	}

	// update the spec tag
	if imageStream.Spec.Tags == nil {
		imageStream.Spec.Tags = map[string]imageapi.TagReference{}
	}
	tagRef, exists := imageStream.Spec.Tags[tag]
	// if the caller set tag, override the spec tag
	if istag.Tag != nil {
		tagRef = *istag.Tag
		tagRef.Name = tag
	}
	tagRef.Annotations = istag.Annotations
	imageStream.Spec.Tags[tag] = tagRef

	// mutate the image stream
	var newImageStream *imageapi.ImageStream
	if create {
		newImageStream, err = r.imageStreamRegistry.CreateImageStream(ctx, imageStream)
	} else {
		newImageStream, err = r.imageStreamRegistry.UpdateImageStream(ctx, imageStream)
	}
	if err != nil {
		return nil, false, err
	}

	image, err := r.imageFor(ctx, tag, newImageStream)
	if err != nil {
		if !kapierrors.IsNotFound(err) {
			return nil, false, err
		}
	}

	newISTag, err := newISTag(tag, newImageStream, image, true)
	return newISTag, !exists, err
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
		return nil, kapierrors.NewNotFound(imageapi.Resource("imagestreamtags"), tag)
	}

	if _, err = r.imageStreamRegistry.UpdateImageStream(ctx, stream); err != nil {
		return nil, fmt.Errorf("cannot remove tag from image stream: %v", err)
	}

	return &unversioned.Status{Status: unversioned.StatusSuccess}, nil
}

// imageFor retrieves the most recent image for a tag in a given imageStreem.
func (r *REST) imageFor(ctx kapi.Context, tag string, imageStream *imageapi.ImageStream) (*imageapi.Image, error) {
	event := imageapi.LatestTaggedImage(imageStream, tag)
	if event == nil || len(event.Image) == 0 {
		return nil, kapierrors.NewNotFound(imageapi.Resource("imagestreamtags"), imageapi.JoinImageStreamTag(imageStream.Name, tag))
	}

	return r.imageRegistry.GetImage(ctx, event.Image)
}

// newISTag initializes an image stream tag from an image stream and image. The allowEmptyEvent will create a tag even
// in the event that the status tag does does not exist yet (no image has successfully been tagged) or the image is nil.
func newISTag(tag string, imageStream *imageapi.ImageStream, image *imageapi.Image, allowEmptyEvent bool) (*imageapi.ImageStreamTag, error) {
	istagName := imageapi.JoinImageStreamTag(imageStream.Name, tag)

	event := imageapi.LatestTaggedImage(imageStream, tag)
	if event == nil || len(event.Image) == 0 {
		if !allowEmptyEvent {
			return nil, kapierrors.NewNotFound(imageapi.Resource("imagestreamtags"), istagName)
		}
		event = &imageapi.TagEvent{
			Created: imageStream.CreationTimestamp,
		}
	}

	ist := &imageapi.ImageStreamTag{
		ObjectMeta: kapi.ObjectMeta{
			Namespace:         imageStream.Namespace,
			Name:              istagName,
			CreationTimestamp: event.Created,
			Annotations:       map[string]string{},
			ResourceVersion:   imageStream.ResourceVersion,
			UID:               imageStream.UID,
		},
		Generation: event.Generation,
		Conditions: imageStream.Status.Tags[tag].Conditions,
	}

	if imageStream.Spec.Tags != nil {
		if tagRef, ok := imageStream.Spec.Tags[tag]; ok {
			// copy the spec tag
			ist.Tag = &tagRef
			if from := ist.Tag.From; from != nil {
				copied := *from
				ist.Tag.From = &copied
			}
			if gen := ist.Tag.Generation; gen != nil {
				copied := *gen
				ist.Tag.Generation = &copied
			}

			// if the imageStream has Spec.Tags[tag].Annotations[k] = v, copy it to the image's annotations
			// and add them to the istag's annotations
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
		if err := imageapi.ImageWithMetadata(image); err != nil {
			return nil, err
		}
		image.DockerImageManifest = ""
		ist.Image = *image
	} else {
		ist.Image = imageapi.Image{}
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
