package imagesignature

import (
	"errors"

	kapierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	apirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"

	"github.com/openshift/origin/pkg/client"
	imageapi "github.com/openshift/origin/pkg/image/apis/image"
)

// REST implements the RESTStorage interface for ImageSignature
type REST struct {
	imageClient client.ImageInterface
}

// NewREST returns a new REST.
func NewREST(imageClient client.ImageInterface) *REST {
	return &REST{imageClient: imageClient}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &imageapi.ImageSignature{}
}

func (r *REST) Create(ctx apirequest.Context, obj runtime.Object, _ bool) (runtime.Object, error) {
	signature := obj.(*imageapi.ImageSignature)

	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}

	imageName, _, err := imageapi.SplitImageSignatureName(signature.Name)
	if err != nil {
		return nil, kapierrors.NewBadRequest(err.Error())
	}

	image, err := r.imageClient.Get(imageName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// ensure that given signature already doesn't exist - either by its name or type:content
	if byName, byContent := imageapi.IndexOfImageSignatureByName(image.Signatures, signature.Name), imageapi.IndexOfImageSignature(image.Signatures, signature.Type, signature.Content); byName >= 0 || byContent >= 0 {
		return nil, kapierrors.NewAlreadyExists(imageapi.Resource("imageSignatures"), signature.Name)
	}

	image.Signatures = append(image.Signatures, *signature)

	image, err = r.imageClient.Update(image)
	if err != nil {
		return nil, err
	}

	byName := imageapi.IndexOfImageSignatureByName(image.Signatures, signature.Name)
	if byName < 0 {
		return nil, kapierrors.NewInternalError(errors.New("failed to store given signature"))
	}

	return &image.Signatures[byName], nil
}

func (r *REST) Delete(ctx apirequest.Context, name string) (runtime.Object, error) {
	imageName, _, err := imageapi.SplitImageSignatureName(name)
	if err != nil {
		return nil, kapierrors.NewBadRequest("ImageSignatures must be accessed with <imageName>@<signatureName>")
	}

	image, err := r.imageClient.Get(imageName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	index := imageapi.IndexOfImageSignatureByName(image.Signatures, name)
	if index < 0 {
		return nil, kapierrors.NewNotFound(imageapi.Resource("imageSignatures"), name)
	}

	size := len(image.Signatures)
	copy(image.Signatures[index:size-1], image.Signatures[index+1:size])
	image.Signatures = image.Signatures[0 : size-1]

	if _, err := r.imageClient.Update(image); err != nil {
		return nil, err
	}

	return &metav1.Status{Status: metav1.StatusSuccess}, nil
}
