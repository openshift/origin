package image

import (
	"fmt"
	"strings"

	"github.com/golang/glog"

	kapi "k8s.io/kubernetes/pkg/api"
	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/util/sets"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// internalRegistryNames is a set of all registry names referencing internal docker registry.
var internalRegistryNames sets.String

func init() {
	internalRegistryNames = sets.NewString()
}

// GenericImageStreamUsageComputer allows to compute number of images stored
// in an internal registry in particular namespace.
type GenericImageStreamUsageComputer struct {
	osClient osclient.Interface
	// says whether to account for images stored in image stream's spec
	processSpec bool
	// Whether to verify that referenced image belongs to its supposed source image stream when fetching it
	// from etcd. Set to true if the image stream corresponds to its counterpart stored in etcd.
	fetchImagesFromImageStream bool
	// maps image name to an image object. It holds only images stored in the registry to avoid multiple
	// fetches of the same object.
	imageCache map[string]*imageapi.Image
	// maps image stream name prefixed by its namespace to the image stream object
	imageStreamCache map[string]*imageapi.ImageStream
}

// NewGenericImageStreamUsageComputer returns an instance of GenericImageStreamUsageComputer.
// Returned object can be used just once and must be thrown away afterwards.
func NewGenericImageStreamUsageComputer(osClient osclient.Interface, processSpec bool, fetchImagesFromImageStream bool) *GenericImageStreamUsageComputer {
	return &GenericImageStreamUsageComputer{
		osClient,
		processSpec,
		fetchImagesFromImageStream,
		make(map[string]*imageapi.Image),
		make(map[string]*imageapi.ImageStream),
	}
}

// GetImageStreamUsage counts number of unique internally managed images occupying given image stream. Each
// Images given in processedImages won't be taken into account. The set will be updated with new images found.
func (c *GenericImageStreamUsageComputer) GetImageStreamUsage(
	is *imageapi.ImageStream,
	processedImages sets.String,
) *resource.Quantity {
	images := resource.NewQuantity(0, resource.DecimalSI)

	c.processImageStreamImages(is, func(_, _ string, img *imageapi.Image) error {
		if processedImages.Has(img.Name) {
			return nil
		}
		processedImages.Insert(img.Name)
		images.Set(images.Value() + 1)
		return nil
	})

	return images
}

// GetProjectImagesUsage returns a number of internally managed images tagged in the given namespace.
func (c *GenericImageStreamUsageComputer) GetProjectImagesUsage(namespace string) (*resource.Quantity, error) {
	processedImages := sets.NewString()

	iss, err := c.listImageStreams(namespace)
	if err != nil {
		return nil, err
	}

	images := resource.NewQuantity(0, resource.DecimalSI)

	for _, is := range iss.Items {
		c.processImageStreamImages(&is, func(_, _ string, img *imageapi.Image) error {
			if !processedImages.Has(img.Name) {
				processedImages.Insert(img.Name)
				images.Set(images.Value() + 1)
			}
			return nil
		})
	}

	return images, err
}

// GetProjectImagesUsageIncrement computes image count in the namespace for given image
// stream (new or updated) and new image. It returns:
//
//  1. number of images currently tagged in the namespace; the image and images tagged in the given is don't
//     count unless they are tagged in other is as well
//  2. number of new internally managed images referenced either by the is or by the image
//  3. an error if something goes wrong
func (c *GenericImageStreamUsageComputer) GetProjectImagesUsageIncrement(
	namespace string,
	is *imageapi.ImageStream,
	image *imageapi.Image,
) (images, imagesIncrement *resource.Quantity, err error) {
	processedImages := sets.NewString()

	iss, err := c.listImageStreams(namespace)
	if err != nil {
		return
	}

	imagesIncrement = resource.NewQuantity(0, resource.DecimalSI)

	for _, imageStream := range iss.Items {
		if is != nil && imageStream.Name == is.Name {
			continue
		}
		c.processImageStreamImages(&imageStream, func(_, _ string, img *imageapi.Image) error {
			processedImages.Insert(img.Name)
			return nil
		})
	}

	if is != nil {
		c.processImageStreamImages(is, func(_, _ string, img *imageapi.Image) error {
			if !processedImages.Has(img.Name) {
				processedImages.Insert(img.Name)
				imagesIncrement.Set(imagesIncrement.Value() + 1)
			}
			return nil
		})
	}

	if image != nil && !processedImages.Has(image.Name) {
		if value, ok := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; ok && value == "true" {
			if !processedImages.Has(image.Name) {
				imagesIncrement.Set(imagesIncrement.Value() + 1)
			}
		}
	}

	images = resource.NewQuantity(int64(len(processedImages)), resource.DecimalSI)

	return
}

// processImageStreamImages is a utility method that calls a given handler for every image of the given image
// stream that belongs to internal registry. It process image stream status and optionally spec.
func (c *GenericImageStreamUsageComputer) processImageStreamImages(
	is *imageapi.ImageStream,
	handler func(tag string, dockerImageReference string, image *imageapi.Image) error,
) error {
	processedImages := sets.NewString()

	for tag, history := range is.Status.Tags {
		for i := range history.Items {
			imageName := history.Items[i].Image
			if len(history.Items[i].DockerImageReference) == 0 || len(imageName) == 0 {
				continue
			}

			var (
				err error
				img *imageapi.Image
			)
			if c.fetchImagesFromImageStream {
				img, err = c.getImageStreamImage(is.Namespace, is.Name+"@"+imageName)
				if err != nil {
					glog.Errorf("Failed to get image %s of image stream %s/%s: %v", imageName, is.Namespace, is.Name, err)
					continue
				}
			} else {
				img, err = c.getImage(imageName)
				if err != nil {
					glog.Errorf("Failed to get image %s: %v", imageName, err)
					continue
				}
			}

			if processedImages.Has(imageName) {
				glog.V(4).Infof("Skipping image %q - already processed", imageName)
				continue
			}
			processedImages.Insert(imageName)

			if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
				glog.V(5).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
				continue
			}

			cacheInternalRegistryName(img.DockerImageReference)

			if err = handler(tag, history.Items[i].DockerImageReference, img); err != nil {
				return err
			}
		}
	}

	if c.processSpec {
		return c.processImageStreamSpecImages(is, processedImages, handler)
	}

	return nil
}

// processImageStreamSpecImages is a utility method that calls a given handler on every image of the given image
// stream that belongs to internal registry. It process image stream's spec only.
func (c *GenericImageStreamUsageComputer) processImageStreamSpecImages(
	is *imageapi.ImageStream,
	processedImages sets.String,
	handler func(tag string, dockerImageReference string, image *imageapi.Image) error,
) error {
	for tag, tagRef := range is.Spec.Tags {
		if tagRef.From == nil {
			continue
		}

		ref, err := c.getImageReferenceForObjectReference(is.Namespace, tagRef.From)
		if err != nil {
			glog.V(4).Infof("Could not process object reference: %v", err)
			continue
		}

		img, exists := c.imageCache[ref.ID]
		if !exists {
			if c.fetchImagesFromImageStream || len(ref.ID) == 0 {
				if len(ref.ID) > 0 {
					img, err = c.getImageStreamImage(ref.Namespace, is.Name+"@"+ref.ID)
					if err != nil {
						glog.Errorf("Failed to get image stream image %s/%s@%s: %v", ref.Namespace, ref.Name, ref.ID, err)
						continue
					}

				} else {
					tag = ref.Tag
					if len(tag) == 0 {
						tag = "latest"
					}
					ist, err2 := c.osClient.ImageStreamTags(ref.Namespace).Get(ref.Name, tag)
					if err2 != nil {
						glog.Errorf("Failed to get image stream tag %s/%s:%s: %v", ref.Namespace, ref.Name, tag, err2)
						continue
					}
					img = &ist.Image
					c.imageCache[ref.ID] = img
				}

			} else {
				img, err = c.getImage(ref.ID)
				if err != nil {
					glog.Errorf("Failed to get image %s: %v", ref.ID, err)
					continue
				}
			}
		}

		if processedImages.Has(img.Name) {
			glog.V(4).Infof("Skipping image %q - already processed", img.Name)
			continue
		}
		processedImages.Insert(img.Name)

		if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
			glog.V(4).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
			continue
		}

		cacheInternalRegistryName(img.DockerImageReference)

		if err = handler(tag, img.DockerImageReference, img); err != nil {
			return err
		}
	}

	return nil
}

// getImageReferenceForObjectReference returns corresponding docker image reference for the given object
// reference representing either an image stream image or image stream tag or docker image.
func (c *GenericImageStreamUsageComputer) getImageReferenceForObjectReference(
	namespace string,
	objRef *kapi.ObjectReference,
) (imageapi.DockerImageReference, error) {
	switch objRef.Kind {
	case "ImageStreamImage":
		nameParts := strings.Split(objRef.Name, "@")
		if len(nameParts) != 2 {
			return imageapi.DockerImageReference{}, fmt.Errorf("failed to parse name of imageStreamImage %q", objRef.Name)
		}
		res, err := imageapi.ParseDockerImageReference(objRef.Name)
		if err != nil {
			return imageapi.DockerImageReference{}, err
		}
		if res.Namespace == "" {
			res.Namespace = objRef.Namespace
		}
		if res.Namespace == "" {
			res.Namespace = namespace
		}
		return res, nil

	case "ImageStreamTag":
		// This is really fishy. An admission check can be easily worked around by setting a tag reference
		// to an ImageStreamTag with no or small image and then tagging a large image to the source tag.
		// TODO: Shall we refuse an ImageStreamTag set in the spec if the quota is set?
		nameParts := strings.Split(objRef.Name, ":")
		if len(nameParts) != 2 {
			return imageapi.DockerImageReference{}, fmt.Errorf("failed to parse name of imageStreamTag %q", objRef.Name)
		}

		ns := namespace
		if len(objRef.Namespace) > 0 {
			ns = objRef.Namespace
		}

		isName := nameParts[0]
		is, err := c.getImageStream(ns, isName)
		if err != nil {
			return imageapi.DockerImageReference{}, fmt.Errorf("failed to get imageStream for ImageStreamTag %s/%s: %v", ns, objRef.Name, err)
		}

		event := imageapi.LatestTaggedImage(is, nameParts[1])
		if event == nil || len(event.DockerImageReference) == 0 {
			return imageapi.DockerImageReference{}, fmt.Errorf("%q is not currently pointing to an image, cannot use it as the source of a tag", objRef.Name)
		}
		return imageapi.ParseDockerImageReference(event.DockerImageReference)

	case "DockerImage":
		managedByOS, ref := imageReferenceBelongsToInternalRegistry(objRef.Name)
		if !managedByOS {
			return imageapi.DockerImageReference{}, fmt.Errorf("DockerImage %s does not belong to internal registry", objRef.Name)
		}
		return ref, nil
	}

	return imageapi.DockerImageReference{}, fmt.Errorf("unsupported object reference kind %s", objRef.Kind)
}

// getImageStream gets an image stream object from etcd and caches the result for the following queries.
func (c *GenericImageStreamUsageComputer) getImageStream(namespace, name string) (*imageapi.ImageStream, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if is, exists := c.imageStreamCache[key]; exists {
		return is, nil
	}
	is, err := c.osClient.ImageStreams(namespace).Get(name)
	if err == nil {
		c.imageStreamCache[key] = is
	}
	return is, err
}

// getImage gets image object from etcd and caches the result for the following queries.
func (c *GenericImageStreamUsageComputer) getImage(name string) (*imageapi.Image, error) {
	if image, exists := c.imageCache[name]; exists {
		return image, nil
	}
	image, err := c.osClient.Images().Get(name)
	if err == nil {
		c.imageCache[name] = image
	}
	return image, err
}

// getImageStreamImage gets an image belonging to a given image stream object from etcd and caches the result for the following queries.
func (c *GenericImageStreamUsageComputer) getImageStreamImage(namespace, name string) (*imageapi.Image, error) {
	nameParts := strings.SplitN(name, "@", 2)
	if len(nameParts) != 2 {
		return nil, fmt.Errorf("failed to parse image stream image name %q, expected exactly one '@'", name)
	}
	image, exists := c.imageCache[nameParts[1]]
	if exists {
		return image, nil
	}
	isi, err := c.osClient.ImageStreamImages(namespace).Get(nameParts[0], nameParts[1])
	if err == nil {
		image = &isi.Image
		c.imageCache[name] = image
	}
	return image, err
}

// listImageStreams returns a list of image streams of the given namespace and caches them for later access.
func (c *GenericImageStreamUsageComputer) listImageStreams(namespace string) (*imageapi.ImageStreamList, error) {
	iss, err := c.osClient.ImageStreams(namespace).List(kapi.ListOptions{})
	if err == nil {
		for _, is := range iss.Items {
			c.imageStreamCache[fmt.Sprintf("%s/%s", namespace, is.Name)] = &is
		}
	}
	return iss, err
}

// cacheInternalRegistryName caches registry name of the given docker image reference of an image stored in an
// internal registry.
func cacheInternalRegistryName(dockerImageReference string) {
	ref, err := imageapi.ParseDockerImageReference(dockerImageReference)
	if err == nil && len(ref.Registry) > 0 {
		internalRegistryNames.Insert(ref.Registry)
	}
}

// imageReferenceBelongsToInternalRegistry returns true if the given docker image reference refers to an
// image in an internal registry.
func imageReferenceBelongsToInternalRegistry(dockerImageReference string) (bool, imageapi.DockerImageReference) {
	ref, err := imageapi.ParseDockerImageReference(dockerImageReference)
	if err != nil || len(ref.Registry) == 0 || len(ref.Namespace) == 0 || len(ref.Name) == 0 {
		return false, ref
	}
	return internalRegistryNames.Has(ref.Registry), ref
}
