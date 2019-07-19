package imagesignature

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apiserver/pkg/registry/rest"

	imagegroup "github.com/openshift/api/image"
	imagev1 "github.com/openshift/api/image/v1"
	imagev1typedclient "github.com/openshift/client-go/image/clientset/versioned/typed/image/v1"
	imageapi "github.com/openshift/openshift-apiserver/pkg/image/apis/image"
	imageconversions "github.com/openshift/openshift-apiserver/pkg/image/apis/image/v1"
)

// REST implements the RESTStorage interface for ImageSignature
type REST struct {
	imageClient imagev1typedclient.ImagesGetter
}

var _ rest.Creater = &REST{}
var _ rest.GracefulDeleter = &REST{}
var _ rest.Scoper = &REST{}

// NewREST returns a new REST.
func NewREST(imageClient imagev1typedclient.ImagesGetter) *REST {
	return &REST{imageClient: imageClient}
}

// New is only implemented to make REST implement RESTStorage
func (r *REST) New() runtime.Object {
	return &imageapi.ImageSignature{}
}

func (s *REST) NamespaceScoped() bool {
	return false
}

func (r *REST) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc, options *metav1.CreateOptions) (runtime.Object, error) {
	signature := obj.(*imageapi.ImageSignature)

	if err := rest.BeforeCreate(Strategy, ctx, obj); err != nil {
		return nil, err
	}
	// at this point we have a fully formed object.  It is time to call the validators that the apiserver
	// handling chain wants to enforce.
	if createValidation != nil {
		if err := createValidation(obj.DeepCopyObject()); err != nil {
			return nil, err
		}
	}

	imageName, _, err := splitImageSignatureName(signature.Name)
	if err != nil {
		return nil, apierrors.NewBadRequest(err.Error())
	}

	image, err := r.imageClient.Images().Get(imageName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	// ensure that given signature already doesn't exist - either by its name or type:content
	if byName, byContent := indexOfImageSignatureByName(image.Signatures, signature.Name), indexOfImageSignature(image.Signatures, signature.Type, signature.Content); byName >= 0 || byContent >= 0 {
		return nil, apierrors.NewAlreadyExists(imagegroup.Resource("imageSignatures"), signature.Name)
	}

	externalSignature := &imagev1.ImageSignature{}
	if err := imageconversions.Convert_image_ImageSignature_To_v1_ImageSignature(signature, externalSignature, nil); err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	image.Signatures = append(image.Signatures, *externalSignature)

	image, err = r.imageClient.Images().Update(image)
	if err != nil {
		return nil, err
	}

	byName := indexOfImageSignatureByName(image.Signatures, signature.Name)
	if byName < 0 {
		return nil, apierrors.NewInternalError(errors.New("failed to store given signature"))
	}

	return &image.Signatures[byName], nil
}

func (r *REST) Delete(ctx context.Context, name string, options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	imageName, _, err := splitImageSignatureName(name)
	if err != nil {
		return nil, false, apierrors.NewBadRequest("ImageSignatures must be accessed with <imageName>@<signatureName>")
	}

	image, err := r.imageClient.Images().Get(imageName, metav1.GetOptions{})
	if err != nil {
		return nil, false, err
	}

	index := indexOfImageSignatureByName(image.Signatures, name)
	if index < 0 {
		return nil, false, apierrors.NewNotFound(imagegroup.Resource("imageSignatures"), name)
	}

	size := len(image.Signatures)
	copy(image.Signatures[index:size-1], image.Signatures[index+1:size])
	image.Signatures = image.Signatures[0 : size-1]

	if _, err := r.imageClient.Images().Update(image); err != nil {
		return nil, false, err
	}

	return &metav1.Status{Status: metav1.StatusSuccess}, true, nil
}

// IndexOfImageSignatureByName returns an index of signature identified by name in the image if present. It
// returns -1 otherwise.
func indexOfImageSignatureByName(signatures []imagev1.ImageSignature, name string) int {
	for i := range signatures {
		if signatures[i].Name == name {
			return i
		}
	}
	return -1
}

// IndexOfImageSignature returns index of signature identified by type and blob in the image if present. It
// returns -1 otherwise.
func indexOfImageSignature(signatures []imagev1.ImageSignature, sType string, sContent []byte) int {
	for i := range signatures {
		if signatures[i].Type == sType && bytes.Equal(signatures[i].Content, sContent) {
			return i
		}
	}
	return -1
}

// splitImageSignatureName splits given signature name into image name and signature name.
func splitImageSignatureName(imageSignatureName string) (imageName, signatureName string, err error) {
	segments := strings.Split(imageSignatureName, "@")
	switch len(segments) {
	case 2:
		signatureName = segments[1]
		imageName = segments[0]
		if len(imageName) == 0 || len(signatureName) == 0 {
			err = fmt.Errorf("image signature name %q must have an image name and signature name", imageSignatureName)
		}
	default:
		err = fmt.Errorf("expected exactly one @ in the image signature name %q", imageSignatureName)
	}
	return
}
