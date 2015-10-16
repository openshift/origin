package util

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/api"
)

// ImageFinalized returns true if the finalizers does not contain the origin finalizer
func ImageFinalized(image *api.Image) bool {
	for i := range image.Finalizers {
		if oapi.FinalizerOrigin == image.Finalizers[i] {
			return false
		}
	}
	return true
}

// FinalizeImage will remove the origin finalizer from the image
func FinalizeImage(oClient client.ImagesInterfacer, image *api.Image) (result *api.Image, err error) {
	if ImageFinalized(image) {
		return image, nil
	}

	// there is a potential for a resource conflict with base kubernetes finalizer
	// as a result, we handle resource conflicts in case multiple finalizers try
	// to finalize at same time
	for {
		result, err = finalizeImageInternal(oClient, image, false)
		if err == nil {
			return result, nil
		}

		if !kerrors.IsConflict(err) {
			return nil, err
		}

		image, err = oClient.Images().Get(image.Name)
		if err != nil {
			return nil, err
		}
	}
}

// finalizeImageInternal will update the image finalizer list to either have or not have origin finalizer
func finalizeImageInternal(oClient client.ImagesInterfacer, image *api.Image, withOrigin bool) (*api.Image, error) {
	imageFinalize := api.Image{}
	imageFinalize.ObjectMeta = image.ObjectMeta
	imageFinalize.Finalizers = image.Finalizers

	finalizerSet := sets.NewString()
	for i := range image.Finalizers {
		finalizerSet.Insert(string(image.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(oapi.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(oapi.FinalizerOrigin))
	}

	imageFinalize.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		imageFinalize.Finalizers = append(imageFinalize.Finalizers, kapi.FinalizerName(value))
	}
	return oClient.Images().Finalize(&imageFinalize)
}

// ImageStreamFinalized returns true if the spec.finalizers does not contain the origin finalizer
func ImageStreamFinalized(imageStream *api.ImageStream) bool {
	for i := range imageStream.Spec.Finalizers {
		if oapi.FinalizerOrigin == imageStream.Spec.Finalizers[i] {
			return false
		}
	}
	return true
}

// FinalizeImageStream will remove the origin finalizer from the image stream
func FinalizeImageStream(oClient client.ImageStreamsNamespacer, imageStream *api.ImageStream) (result *api.ImageStream, err error) {
	if ImageStreamFinalized(imageStream) {
		return imageStream, nil
	}

	// there is a potential for a resource conflict with base kubernetes finalizer
	// as a result, we handle resource conflicts in case multiple finalizers try
	// to finalize at same time
	for {
		result, err = finalizeImageStreamInternal(oClient, imageStream, false)
		if err == nil {
			return result, nil
		}

		if !kerrors.IsConflict(err) {
			return nil, err
		}

		imageStream, err = oClient.ImageStreams(imageStream.Namespace).Get(imageStream.Name)
		if err != nil {
			return nil, err
		}
	}
}

// finalizeImageStreamInternal will update the image stream finalizer list to either have or not have origin finalizer
func finalizeImageStreamInternal(oClient client.ImageStreamsNamespacer, imageStream *api.ImageStream, withOrigin bool) (*api.ImageStream, error) {
	imageStreamFinalize := api.ImageStream{}
	imageStreamFinalize.ObjectMeta = imageStream.ObjectMeta
	imageStreamFinalize.Spec.Finalizers = imageStream.Spec.Finalizers

	finalizerSet := sets.NewString()
	for i := range imageStream.Spec.Finalizers {
		finalizerSet.Insert(string(imageStream.Spec.Finalizers[i]))
	}

	if withOrigin {
		finalizerSet.Insert(string(oapi.FinalizerOrigin))
	} else {
		finalizerSet.Delete(string(oapi.FinalizerOrigin))
	}

	imageStreamFinalize.Spec.Finalizers = make([]kapi.FinalizerName, 0, len(finalizerSet))
	for _, value := range finalizerSet.List() {
		imageStreamFinalize.Spec.Finalizers = append(imageStreamFinalize.Spec.Finalizers, kapi.FinalizerName(value))
	}
	return oClient.ImageStreams(imageStream.Namespace).Finalize(&imageStreamFinalize)
}
