package imagestreamimage

import (
	"fmt"
	"strings"

	"github.com/docker/distribution/digest"
	"github.com/golang/glog"
	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/fields"
	"k8s.io/kubernetes/pkg/labels"
	"k8s.io/kubernetes/pkg/registry/generic"
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
		return nil, errors.NewNotFound("imageStreamImage", imageID)
	}

	set := api.ResolveImageID(repo, imageID)
	switch len(set) {
	case 1:
		imageName := set.List()[0]
		image, err := r.imageRegistry.GetImage(ctx, imageName)
		if err != nil {
			return nil, err
		}
		imageWithMetadata, err := api.ImageWithMetadata(*image)
		if err != nil {
			return nil, err
		}

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
			Image: *imageWithMetadata,
		}

		return &isi, nil
	case 0:
		return nil, errors.NewNotFound("imageStreamImage", imageID)
	default:
		return nil, errors.NewConflict("imageStreamImage", imageID, fmt.Errorf("multiple images match the prefix %q: %s", imageID, strings.Join(set.List(), ", ")))
	}
}

// NewList returns a new list object
func (r *REST) NewList() runtime.Object {
	return &api.ImageStreamImageList{}
}

func (r *REST) List(ctx kapi.Context, label labels.Selector, field fields.Selector) (runtime.Object, error) {
	repos, err := r.imageStreamRegistry.ListImageStreams(ctx, labels.Everything())
	if err != nil {
		return nil, err
	}

	list := api.ImageStreamImageList{}
	for _, repo := range repos.Items {
		if repo.Status.Tags == nil {
			continue
		}

		for _, history := range repo.Status.Tags {
			for _, tagging := range history.Items {
				imageName := tagging.Image
				image, err := r.imageRegistry.GetImage(ctx, imageName)
				if err != nil {
					glog.V(4).Infof("Failed to get image %q of image stream %s: %v", imageName, repo.Name, err)
					continue
				}
				imageWithMetadata, err := api.ImageWithMetadata(*image)
				if err != nil {
					glog.V(4).Infof("Failed to get metadata of image %q of image stream %s: %v", imageName, repo.Name, err)
					continue
				}

				if d, err := digest.ParseDigest(imageName); err == nil {
					imageName = d.Hex()
				}
				if len(imageName) > 7 {
					imageName = imageName[:7]
				}

				isi := api.ImageStreamImage{
					ObjectMeta: kapi.ObjectMeta{
						Namespace: repo.Namespace,
						Name:      fmt.Sprintf("%s@%s", repo.Name, imageName),
					},
					Image: *imageWithMetadata,
				}

				list.Items = append(list.Items, isi)
			}
		}
	}

	return generic.FilterList(&list, MatchImageStreamImage(label, field), generic.DecoratorFunc(nil))
}

// MatchImageStreamImage returns a generic matcher for a given label and field selector.
func MatchImageStreamImage(label labels.Selector, field fields.Selector) generic.Matcher {
	return generic.MatcherFunc(func(obj runtime.Object) (bool, error) {
		ir, ok := obj.(*api.ImageStreamImage)
		if !ok {
			return false, fmt.Errorf("not an ImageStreamImage")
		}
		fields := ImageStreamImageToSelectableFields(ir)
		return label.Matches(labels.Set(ir.Labels)) && field.Matches(fields), nil
	})
}

// ImageStreamImageToSelectableFields returns a label set that represents the object.
func ImageStreamImageToSelectableFields(ir *api.ImageStreamImage) labels.Set {
	nameParts := strings.Split(ir.Name, "@")
	return labels.Set{
		"metadata.name":      ir.Name,
		"imagestream.name":   nameParts[0],
		"image.name":         ir.Image.Name,
		"image.status.phase": ir.Image.Status.Phase,
	}
}
