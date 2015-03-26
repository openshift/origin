package imagerepositorytag

import (
	kapi "github.com/GoogleCloudPlatform/kubernetes/pkg/api"
	"github.com/GoogleCloudPlatform/kubernetes/pkg/runtime"

	"github.com/openshift/origin/pkg/image/api"
	"github.com/openshift/origin/pkg/image/registry/imagestreamtag"
)

// REST implements the RESTStorage interface for ImageRepositoryTag.
type REST struct {
	imageStreamTagRegistry imagestreamtag.Registry
}

// NewREST returns a new REST.
func NewREST(r imagestreamtag.Registry) *REST {
	return &REST{r}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &api.Image{}
}

// Get retrieves an image that has been tagged by repo and tag. `id` is of the format
// <repo name>:<tag>.
func (r *REST) Get(ctx kapi.Context, id string) (runtime.Object, error) {
	return r.imageStreamTagRegistry.GetImageStreamTag(ctx, id)
}

// Delete removes a tag from a repo. `id` is of the format <repo name>:<tag>.
// The associated image that the tag points to is *not* deleted.
// The tag history remains intact and is not deleted.
func (r *REST) Delete(ctx kapi.Context, id string) (runtime.Object, error) {
	return r.imageStreamTagRegistry.DeleteImageStreamTag(ctx, id)
}
