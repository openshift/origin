package image

import (
	"github.com/golang/glog"

	"k8s.io/kubernetes/pkg/api/resource"
	"k8s.io/kubernetes/pkg/util/sets"

	osclient "github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/api"
)

// GetImageStreamSize computes a sum of sizes of image layers occupying given image stream. Each layer
// is added just once even if it occurs in multiple images.
func GetImageStreamSize(osClient osclient.Interface, is *imageapi.ImageStream, imageCache map[string]*imageapi.Image) *resource.Quantity {
	size := resource.NewQuantity(0, resource.BinarySI)

	processedImages := make(map[string]sets.Empty)
	processedLayers := make(map[string]sets.Empty)

	for _, history := range is.Status.Tags {
		for i := range history.Items {
			imgName := history.Items[i].Image
			if len(history.Items[i].DockerImageReference) == 0 || len(imgName) == 0 {
				continue
			}

			if _, exists := processedImages[imgName]; exists {
				continue
			}
			processedImages[imgName] = sets.Empty{}

			img, exists := imageCache[imgName]
			if !exists {
				imi, err := osClient.ImageStreamImages(is.Namespace).Get(is.Name, imgName)
				if err != nil {
					glog.Errorf("Failed to get image %s of image stream %s/%s: %v", imgName, is.Namespace, is.Name, err)
					continue
				}
				img = &imi.Image
				imageCache[imgName] = img
			}

			if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
				glog.V(5).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
				continue
			}

			if len(img.DockerImageLayers) == 0 || img.DockerImageMetadata.Size == 0 {
				if err := imageapi.ImageWithMetadata(img); err != nil {
					glog.Errorf("Failed to parse metadata of image %q with DockerImageReference %q: %v", img.Name, img.DockerImageReference, err)
					continue
				}
			}

			for _, layer := range img.DockerImageLayers {
				if _, ok := processedLayers[layer.Name]; ok {
					continue
				}
				size.Add(*resource.NewQuantity(layer.Size, resource.BinarySI))
				processedLayers[layer.Name] = sets.Empty{}
			}
		}
	}

	return size
}

// GetImageStreamSizeIncrement computes a sum of sizes of image layers occupying given image stream.
// Additionally it returns an increment which will be added to the size when the given image will be tagged
// into the image stream. In case the is is nil, returned size will be 0 and returned increment will equal to
// the image size.
//
// Size increment will always be less or equal to the image size. Less means, that some of image's layers are
// already stored in a registry.
func GetImageStreamSizeIncrement(osClient osclient.Interface, is *imageapi.ImageStream, image *imageapi.Image, imageCache map[string]*imageapi.Image) (isSize, sizeIncrement *resource.Quantity, err error) {
	if len(image.DockerImageLayers) == 0 || image.DockerImageMetadata.Size == 0 {
		if err = imageapi.ImageWithMetadata(image); err != nil {
			glog.Errorf("Failed to parse metadata of image %q with DockerImageReference %q: %v", image.Name, image.DockerImageReference, err)
			return nil, nil, err
		}
	}
	isSize = resource.NewQuantity(0, resource.BinarySI)
	if value, ok := image.Annotations[imageapi.ManagedByOpenShiftAnnotation]; ok && value == "true" {
		sizeIncrement = resource.NewQuantity(image.DockerImageMetadata.Size, resource.BinarySI)
	} else {
		sizeIncrement = resource.NewQuantity(0, resource.BinarySI)
	}
	if is == nil {
		return
	}

	processedImages := make(map[string]sets.Empty)
	processedLayers := make(map[string]sets.Empty)

	for _, history := range is.Status.Tags {
		for i := range history.Items {
			imageName := history.Items[i].Image
			if len(history.Items[i].DockerImageReference) == 0 || len(imageName) == 0 {
				continue
			}

			if _, exists := processedImages[imageName]; exists {
				continue
			}
			processedImages[imageName] = sets.Empty{}

			imi, err2 := osClient.ImageStreamImages(is.Namespace).Get(is.Name, imageName)
			if err2 != nil {
				glog.Errorf("Failed to get image %s of image stream %s/%s: %v", image.Name, is.Namespace, is.Name, err2)
				continue
			}
			img := &imi.Image

			if value, ok := img.Annotations[imageapi.ManagedByOpenShiftAnnotation]; !ok || value != "true" {
				glog.V(5).Infof("Image %q with DockerImageReference %q belongs to an external registry - skipping", img.Name, img.DockerImageReference)
				continue
			}

			if len(img.DockerImageLayers) == 0 || img.DockerImageMetadata.Size == 0 {
				if err2 := imageapi.ImageWithMetadata(img); err2 != nil {
					glog.Errorf("Failed to parse metadata of image %q with DockerImageReference %q: %v", img.Name, img.DockerImageReference, err2)
					continue
				}
			}

			for _, layer := range img.DockerImageLayers {
				if _, exists := processedLayers[layer.Name]; exists {
					continue
				}
				isSize.Add(*resource.NewQuantity(layer.Size, resource.BinarySI))
				processedLayers[layer.Name] = sets.Empty{}
			}
		}
	}

	for _, layer := range image.DockerImageLayers {
		if _, exists := processedLayers[layer.Name]; exists {
			sizeIncrement.Sub(*resource.NewQuantity(layer.Size, resource.BinarySI))
			if sizeIncrement.Value() <= 0 {
				sizeIncrement.Set(0)
				break
			}
		}
	}

	return
}
