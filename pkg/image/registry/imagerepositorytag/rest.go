package imagerepositorytag

import (
	"fmt"
	"strings"

	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/api/errors"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/image"
	"github.com/openshift/origin/pkg/image/registry/imagerepository"
)

// REST implements the RESTStorage interface for ImageRepositoryTag
// It only supports the Get method and is used to simplify retrieving an Image by tag from an ImageRepository
type REST struct {
	imageRegistry           image.Registry
	imageRepositoryRegistry imagerepository.Registry
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageRepositoryRegistry imagerepository.Registry) *REST {
	return &REST{imageRegistry, imageRepositoryRegistry}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.Image{}
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
			err = errors.NewBadRequest("imageRepositoryTags must be retrieved with <name>:<tag>")
		}
	default:
		err = errors.NewBadRequest("imageRepositoryTags must be retrieved with <name>:<tag>")
	}
	return
}

// Get retrieves an image that has been tagged by repo and tag. `id` is of the format
// <repo name>:<tag>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	repo, err := r.imageRepositoryRegistry.GetImageRepository(ctx, name)
	if err != nil {
		return nil, err
	}

	event, err := api.LatestTaggedImage(repo, tag)
	if err != nil {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}

	if len(event.Image) != 0 {
		image, err := r.imageRegistry.GetImage(ctx, event.Image)
		if err != nil {
			return nil, err
		}
		return api.ImageWithMetadata(*image)
	}
	if len(event.DockerImageReference) == 0 {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}

	return &api.Image{
		ObjectMeta: kapi.ObjectMeta{
			CreationTimestamp: event.Created,
		},
		DockerImageReference: event.DockerImageReference,
	}, nil
}

// Delete removes a tag from a repo. `id` is of the format <repo name>:<tag>.
// The associated image that the tag points to is *not* deleted.
// The tag history remains intact and is not deleted.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	name, tag, err := nameAndTag(id)
	if err != nil {
		return nil, err
	}

	repo, err := r.imageRepositoryRegistry.GetImageRepository(ctx, name)
	if err != nil {
		return nil, err
	}

	if repo.Tags == nil {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}

	_, ok := repo.Tags[tag]
	if !ok {
		return nil, errors.NewNotFound("imageRepositoryTag", tag)
	}

	delete(repo.Tags, tag)

	err = r.imageRepositoryRegistry.UpdateImageRepository(ctx, repo)
	if err != nil {
		return nil, fmt.Errorf("Error removing tag from image repository: %s", err)
	}

	return &kapi.Status{Status: kapi.StatusSuccess}, nil
}
