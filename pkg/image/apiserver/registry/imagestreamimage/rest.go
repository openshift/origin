package imagestreamimage

import (
	"context"

	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
	"k8s.io/kubernetes/pkg/printers"
	printerstorage "k8s.io/kubernetes/pkg/printers/storage"

	imagegroup "github.com/openshift/api/image"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
	"github.com/openshift/origin/pkg/image/apiserver/registry/image"
	"github.com/openshift/origin/pkg/image/apiserver/registry/imagestream"
	"github.com/openshift/origin/pkg/image/util"
	printersinternal "github.com/openshift/origin/pkg/printers/internalversion"
)

// REST implements the RESTStorage interface in terms of an image registry and
// image stream registry. It only supports the Get method and is used
// to retrieve an image by id, scoped to an ImageStream. REST ensures
// that the requested image belongs to the specified ImageStream.
type REST struct {
	imageRegistry       image.Registry
	imageStreamRegistry imagestream.Registry
	rest.TableConvertor
}

var _ rest.Getter = &REST{}
var _ rest.ShortNamesProvider = &REST{}
var _ rest.Scoper = &REST{}

// ShortNames implements the ShortNamesProvider interface. Returns a list of short names for a resource.
func (r *REST) ShortNames() []string {
	return []string{"isimage"}
}

// NewREST returns a new REST.
func NewREST(imageRegistry image.Registry, imageStreamRegistry imagestream.Registry) *REST {
	return &REST{
		imageRegistry,
		imageStreamRegistry,
		printerstorage.TableConvertor{TablePrinter: printers.NewTablePrinter().With(printersinternal.AddHandlers)},
	}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &imageapi.ImageStreamImage{}
}

func (s *REST) NamespaceScoped() bool {
	return true
}

// parseNameAndID splits a string into its name component and ID component, and returns an error
// if the string is not in the right form.
func parseNameAndID(input string) (name string, id string, err error) {
	name, id, err = imageapi.ParseImageStreamImageName(input)
	if err != nil {
		err = errors.NewBadRequest("ImageStreamImages must be retrieved with <name>@<id>")
	}
	return
}

// Get retrieves an image by ID that has previously been tagged into an image stream.
// `id` is of the form <repo name>@<image id>.
func (r *REST) Get(ctx context.Context, id string, options *metav1.GetOptions) (runtime.Object, error) {
	name, imageID, err := parseNameAndID(id)
	if err != nil {
		return nil, err
	}

	repo, err := r.imageStreamRegistry.GetImageStream(ctx, name, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	if repo.Status.Tags == nil {
		return nil, errors.NewNotFound(imagegroup.Resource("imagestreamimage"), id)
	}

	event, err := imageapi.ResolveImageID(repo, imageID)
	if err != nil {
		return nil, err
	}

	imageName := event.Image
	image, err := r.imageRegistry.GetImage(ctx, imageName, &metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if err := util.InternalImageWithMetadata(image); err != nil {
		return nil, err
	}
	image.DockerImageManifest = ""
	image.DockerImageConfig = ""

	isi := imageapi.ImageStreamImage{
		ObjectMeta: metav1.ObjectMeta{
			Namespace:         apirequest.NamespaceValue(ctx),
			Name:              imageapi.JoinImageStreamImage(name, imageID),
			CreationTimestamp: image.ObjectMeta.CreationTimestamp,
			Annotations:       repo.Annotations,
		},
		Image: *image,
	}

	return &isi, nil
}
