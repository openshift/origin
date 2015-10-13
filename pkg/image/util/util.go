package util

import (
	kapi "k8s.io/kubernetes/pkg/api"
	kerrors "k8s.io/kubernetes/pkg/api/errors"
	"k8s.io/kubernetes/pkg/util/sets"

	oapi "github.com/openshift/origin/pkg/api"
	"github.com/openshift/origin/pkg/client"
	"github.com/openshift/origin/pkg/image/api"
)

// Finalized returns true if the spec.finalizers does not contain the origin finalizer
func Finalized(image *api.Image) bool {
	for i := range image.Finalizers {
		if oapi.FinalizerOrigin == image.Finalizers[i] {
			return false
		}
	}
	return true
}

// Finalize will remove the origin finalizer from the image
func Finalize(oClient client.Interface, image *api.Image) (result *api.Image, err error) {
	if Finalized(image) {
		return image, nil
	}

	// there is a potential for a resource conflict with base kubernetes finalizer
	// as a result, we handle resource conflicts in case multiple finalizers try
	// to finalize at same time
	for {
		result, err = finalizeInternal(oClient, image, false)
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

// finalizeInternal will update the image finalizer list to either have or not have origin finalizer
func finalizeInternal(oClient client.Interface, image *api.Image, withOrigin bool) (*api.Image, error) {
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
